use std::collections::BTreeSet;
use async_graphql::SimpleObject;
use linera_sdk::base::{AccountOwner};
use linera_sdk::views::{MapView, RegisterView, RootView, ViewStorageContext};
use universal_solver::{SolverFileId, SolverFile};

#[derive(RootView, SimpleObject)]
#[view(context = "ViewStorageContext")]
pub struct UniversalSolverState {
    pub files: MapView<SolverFileId, SolverFile>,
    pub owned_files: MapView<AccountOwner, BTreeSet<SolverFileId>>,
    pub pool_list: MapView<String, String>,  // chain_name -> pool_address
    pub pool_balances: MapView<String, String>,
}