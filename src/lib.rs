use std::fmt::{Display, Formatter};
use async_graphql::{InputObject, Request, Response, SimpleObject};
use linera_sdk::base::{AccountOwner, ApplicationId, ChainId, ContractAbi, ServiceAbi};
use serde::{Deserialize, Serialize};
use linera_sdk::{bcs, DataBlobHash, ToBcsBytes};
use linera_sdk::graphql::GraphQLMutationRoot;

pub struct UniversalSolverAbi;


#[derive(
    Debug, Serialize, Deserialize, Clone, PartialEq, Eq, Ord, PartialOrd, SimpleObject, InputObject,
)]
#[graphql(input_name = "SolverAppInput")]
pub struct SolverFileId {
    pub id: Vec<u8>,
}


impl ContractAbi for UniversalSolverAbi {
    type Operation = Operation;
    type Response = ();
}

impl ServiceAbi for UniversalSolverAbi {
    type Query = Request;
    type QueryResponse = Response;
}

#[derive(Debug, Deserialize, Serialize, GraphQLMutationRoot)]
pub enum Operation {
    AddFile {
        owner: AccountOwner,
        name: String,
        blob_hash: DataBlobHash,
    },
    AddPool {
        chain_name: String,
        pool_address: String,
    },
    RemovePool {
        chain_name: String,
    },
    UpdatePoolBalance {
        pool_address: String,
        balance: String,
    },
    Swap {
        from_token: String,
        to_token: String,
        destination_address: String,
        amount: String,
    },
}

#[derive(Debug, Serialize, Deserialize, Clone, SimpleObject, PartialEq, Eq)]
#[serde(rename_all = "camelCase")]
pub struct SolverFile {
    pub solver_file_id: SolverFileId,
    pub owner: AccountOwner,
    pub name: String,
    pub blob_hash: DataBlobHash,
}

#[derive(Debug, Serialize, Deserialize, Clone, SimpleObject, PartialEq, Eq)]
#[serde(rename_all = "camelCase")]
pub struct SolverFileOutput {
    pub solver_file_id: String,
    pub owner: AccountOwner,
    pub name: String,
    pub payload: Vec<u8>,
}

impl SolverFileOutput {
    pub fn new(solver_file: SolverFile, payload: Vec<u8>) -> Self {
        use base64::engine::{general_purpose::STANDARD_NO_PAD, Engine as _};
        let file_id = STANDARD_NO_PAD.encode(solver_file.solver_file_id.id);
        Self {
            solver_file_id: file_id,
            owner: solver_file.owner,
            name: solver_file.name,
            payload
        }
    }

    pub fn new_with_token_id(solver_file_id: String, solver_file: SolverFile, payload: Vec<u8>) -> Self {
        Self{
            solver_file_id,
            owner: solver_file.owner,
            name: solver_file.name,
            payload
        }
    }
}

impl Display for SolverFileId {
    fn fmt(&self, f: &mut Formatter<'_>) -> std::fmt::Result {
        write!(f, "{:?}", self.id)
    }
}

impl SolverFile {
    pub fn create_token_id(
        chain_id: &ChainId,
        app_id: &ApplicationId,
        name: &String,
        blob_hash: &DataBlobHash,

    ) -> Result<SolverFileId, bcs::Error> {
        use sha3::Digest as _;
        let mut hasher = sha3::Sha3_256::new();
        hasher.update(chain_id.to_bcs_bytes()?);
        hasher.update(app_id.to_bcs_bytes()?);
        hasher.update(name);
        hasher.update(name.len().to_bcs_bytes()?);
        hasher.update(blob_hash.to_bcs_bytes()?);
        Ok(
            SolverFileId{
                id: hasher.finalize().to_vec(),
            }
        )

    }
}

pub const ALCHEMY_API_KEY: &str = "oAqlLotGsW9i5DDDa-kcBQVjIgfByLaV";

#[derive(SimpleObject)]
pub struct Pool {
    pub chain_name: String,
    pub pool_address: String,
}

#[derive(SimpleObject)]
pub struct PoolBalance {
    pub pool_address: String,
    pub balance: f64,
}

#[derive(SimpleObject)]
pub struct SwapResult {
    pub from_token: String,
    pub to_token: String,
    pub from_amount: f64,
    pub to_amount: f64,
    pub exchange_rate: f64,
}

