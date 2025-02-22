#![cfg_attr(target_arch = "wasm32", no_main)]
mod state;

use self::state::UniversalSolverState;
use async_graphql::{EmptySubscription, Object, Request, Response, Result, Schema};
use base64::engine::general_purpose::STANDARD_NO_PAD;
use base64::Engine;
use linera_sdk::base::AccountOwner;
use linera_sdk::{base::WithServiceAbi, bcs, ensure, http, DataBlobHash, Service, ServiceRuntime};
use linera_views::views::View;
use serde_json;
use std::collections::BTreeMap;
use std::sync::{Arc, Mutex};
use universal_solver::{Operation, Pool, PoolBalance, SolverFileId, SolverFileOutput, UniversalSolverAbi, ALCHEMY_API_KEY};
use universal_solver::SwapResult;

pub struct UniversalSolverService {
    state: Arc<UniversalSolverState>,
    runtime: Arc<Mutex<ServiceRuntime<Self>>>,
}


linera_sdk::service!(UniversalSolverService);

impl WithServiceAbi for UniversalSolverService {
    type Abi = UniversalSolverAbi;
}

impl Service for UniversalSolverService {
    type Parameters = ();
    async fn new(runtime: ServiceRuntime<Self>) -> Self {
        let state = UniversalSolverState::load(runtime.root_view_storage_context())
            .await
            .expect("Failed to load state");
        UniversalSolverService {
            state: Arc::new(state),
            runtime: Arc::new(Mutex::new(runtime)),
        }
    }

    async fn handle_query(&self, request: Request) -> Response {
        let schema = Schema::build(
            QueryRoot {
                file_solver_app: self.state.clone(),
                runtime: self.runtime.clone(),
            },
            MutationRoot,
            EmptySubscription,
        )
            .finish();
        schema.execute(request).await
    }
}


struct QueryRoot {
    file_solver_app: Arc<UniversalSolverState>,
    runtime: Arc<Mutex<ServiceRuntime<UniversalSolverService>>>,
}

#[Object]
impl QueryRoot {
    async fn get_file_solver_app(&self, id: String) -> Option<SolverFileOutput> {
        let token_id_vec = STANDARD_NO_PAD.decode(&id).unwrap();
        let solver_app = self.file_solver_app.files.get(&SolverFileId {id: token_id_vec}).await.unwrap();
        if let Some(solver_app) = solver_app {
            let data_blob = {
                let mut runtime = self.runtime
                    .try_lock()
                    .expect("Services only run in a single thread");
                runtime.read_data_blob(solver_app.blob_hash)
            };
            let solver_output = SolverFileOutput::new_with_token_id(id, solver_app, data_blob);
            Some(solver_output)
        } else {
            None
        }
    }

    /// Get files by owner
    async fn get_files_by_owner(&self, owner: AccountOwner) ->BTreeMap<String, SolverFileOutput> {
        let mut files = BTreeMap::new();
        let file_token_ids = self
            .file_solver_app
            .owned_files
            .get(&owner)
            .await
            .unwrap();

        for token_id in file_token_ids.into_iter().flatten() {
            let file = self
                .file_solver_app
                .files
                .get(&token_id)
                .await
                .unwrap()
                .unwrap();
            let payload = {
                let mut runtime = self
                    .runtime
                    .try_lock()
                    .expect("Services only run in a single thread");
                runtime.read_data_blob(file.blob_hash)
            };
            let file_output = SolverFileOutput::new(file, payload);
            files.insert(file_output.solver_file_id.clone(), file_output);
        }

        files
    }

    /// Convert JSON string to bytes
    async fn json_to_bytes(&self, json_str: String) -> Result<Vec<u8>> {
        // Parse the JSON string to ensure it's valid
        let _: serde_json::Value = serde_json::from_str(&json_str)
            .map_err(|e| async_graphql::Error::new(format!("Invalid JSON: {}", e)))?;
        
        // Convert to bytes
        Ok(json_str.into_bytes())
    }

    /// Convert bytes to JSON string
    async fn bytes_to_json(&self, bytes: Vec<u8>) -> Result<String> {
        String::from_utf8(bytes)
            .map_err(|e| async_graphql::Error::new(format!("Invalid UTF-8 bytes: {}", e)))
    }

    async fn get_pool(&self, chain_name: String) -> Result<Option<String>> {
        Ok(self.file_solver_app.pool_list.get(&chain_name).await?)
    }

    async fn get_all_pools(&self) -> Result<Vec<Pool>> {
        let mut pools = Vec::new();
        self.file_solver_app.pool_list.for_each_index_value(|chain_name, pool_address| {
            pools.push(Pool {
                chain_name: chain_name.clone(),
                pool_address: pool_address.to_string(),
            });
            Ok(())
        }).await?;
        Ok(pools)
    }

    async fn get_pool_balance(&self, pool_address: String) -> Result<Option<String>> {
        Ok(self.file_solver_app.pool_balances.get(&pool_address).await?)
    }

    async fn get_all_pool_balances(&self) -> Result<Vec<PoolBalance>> {
        let mut balances = Vec::new();
        self.file_solver_app.pool_balances.for_each_index_value(|pool_address, balance| {
            balances.push(PoolBalance {
                pool_address: pool_address.clone(),
                balance: (*balance).parse::<f64>().unwrap(),
            });
            Ok(())
        }).await?;
        Ok(balances)
    }
    async fn calculate_swap(&self,
        from_token: String,
        to_token: String,
        amount: f64,
    ) -> Result<SwapResult> {
        // Verify tokens exist in pool list
        let from_address = self.file_solver_app.pool_list.get(&from_token).await?
            .ok_or_else(|| async_graphql::Error::new("Source token pool not found"))?;
        let _to_address = self.file_solver_app.pool_list.get(&to_token).await?
            .ok_or_else(|| async_graphql::Error::new("Target token pool not found"))?;

        // Check balance
        let from_balance = self.file_solver_app.pool_balances.get(&from_address).await?
            .ok_or_else(|| async_graphql::Error::new("Source balance not found"))?;

        if from_balance.parse::<f64>().unwrap() < amount {
            return Err(async_graphql::Error::new("Insufficient balance"));
        }

        let exchange_rate = self.calculate_rate(from_token.clone(), to_token.clone())?;
        
        let to_amount = amount * exchange_rate;
        
        Ok(SwapResult {
            from_token,
            to_token,
            from_amount: amount,
            to_amount,
            exchange_rate,
        })
    }


}

impl QueryRoot {
    fn calculate_rate(
        &self,
        from_token: String,
        to_token: String,
    ) -> Result<f64> {
        let url = format!(
            "https://api.g.alchemy.com/prices/v1/tokens/by-symbol?symbols={}&symbols={}",
            from_token, to_token
        );

        let mut runtime = self.runtime
            .try_lock()
            .expect("Services only run in a single thread");

        // Make HTTP request using runtime
        let response = runtime.http_request(
            http::Request::get(&url)
                .with_header("Authorization", format!("Bearer {}", ALCHEMY_API_KEY).as_bytes())
                .with_header("accept", b"application/json")
        );

        ensure!(
            response.status == 200,
            async_graphql::Error::new(format!(
                "Failed to query Alchemy API. Status code: {}",
                response.status
            ))
        );

        // Parse response
        let data: serde_json::Value = serde_json::from_slice(&response.body)
            .map_err(|e| async_graphql::Error::new(format!("Failed to parse response: {}", e)))?;

        // Extract and validate prices
        let from_price = data["data"]
            .as_array()
            .and_then(|tokens| tokens.iter().find(|t| t["symbol"].as_str() == Some(&from_token)))
            .and_then(|token| token["prices"][0]["value"].as_str())
            .and_then(|price| price.parse::<f64>().ok())
            .ok_or_else(|| async_graphql::Error::new(format!("Price not found for {}", from_token)))?;

        let to_price = data["data"]
            .as_array() 
            .and_then(|tokens| tokens.iter().find(|t| t["symbol"].as_str() == Some(&to_token)))
            .and_then(|token| token["prices"][0]["value"].as_str())
            .and_then(|price| price.parse::<f64>().ok())
            .ok_or_else(|| async_graphql::Error::new(format!("Price not found for {}", to_token)))?;

        ensure!(
            from_price > 0.0 && to_price > 0.0,
            async_graphql::Error::new("Invalid price values received")
        );

        // Calculate and return exchange rate
        // If from_price is ETH price ($2000) and to_price is SOL price ($14.21)
        // Then rate should be from_price/to_price = 140.72
        // This gives correct conversion: 10 ETH * 140.72 = 140.72 SOL
        Ok(from_price / to_price)
    }
}



struct MutationRoot;

#[Object]
impl MutationRoot {
    async fn add_file(&self, owner: AccountOwner, name: String, blob_hash: DataBlobHash) -> Vec<u8> {
        bcs::to_bytes(&Operation::AddFile {
            owner,
            name,
            blob_hash,
        }).unwrap()
    }

    async fn add_pool(&self, chain_name: String, pool_address: String) -> Vec<u8> {
        bcs::to_bytes(&Operation::AddPool {
            chain_name,
            pool_address,
        }).unwrap()
    }

    async fn remove_pool(&self, chain_name: String) -> Vec<u8> {
        bcs::to_bytes(&Operation::RemovePool {
            chain_name,
        }).unwrap()
    }

    async fn update_pool_balance(&self, pool_address: String, balance: String) -> Result<Vec<u8>> {
        Ok(bcs::to_bytes(&Operation::UpdatePoolBalance {
            pool_address,
            balance,
        }).unwrap())
    }

    async fn swap(
        &self,
        from_token: String,
        to_token: String,
        destination_address: String,
        amount: String
    ) -> Result<Vec<u8>> {
        Ok(bcs::to_bytes(&Operation::Swap {
            from_token,
            to_token,
            destination_address,
            amount,
        }).unwrap())
    }
}





