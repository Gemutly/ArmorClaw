use async_trait::async_trait;
use azure_storage::StorageCredentials;
use azure_storage_blobs::prelude::*;

use crate::error::SidecarError;

use super::CloudConnector;

pub struct AzureBlobConfig {
    pub account_name: String,
    pub account_key: String,
    pub container_name: String,
}

pub struct AzureBlobConnector {
    service_client: BlobServiceClient,
    config: AzureBlobConfig,
}

impl AzureBlobConnector {
    pub fn new(config: AzureBlobConfig) -> Result<Self, SidecarError> {
        let credentials = StorageCredentials::access_key(
            config.account_name.clone(),
            config.account_key.clone(),
        );
        let service_client = BlobServiceClient::new(&config.account_name, credentials);

        Ok(Self {
            service_client,
            config,
        })
    }

    pub async fn upload_blob(&self, blob_name: &str, data: &[u8]) -> Result<String, SidecarError> {
        self.service_client
            .container_client(&self.config.container_name)
            .blob_client(blob_name)
            .put_block_blob(data.to_vec())
            .await
            .map(|_| blob_name.to_string())
            .map_err(|e| SidecarError::CloudStorageError(format!("Upload failed: {}", e)))
    }

    pub async fn download_blob(&self, blob_name: &str) -> Result<Vec<u8>, SidecarError> {
        self.service_client
            .container_client(&self.config.container_name)
            .blob_client(blob_name)
            .get_content()
            .await
            .map_err(|e| SidecarError::CloudStorageError(format!("Download failed: {}", e)))
    }

    pub async fn list_blobs(&self, prefix: &str) -> Result<Vec<String>, SidecarError> {
        let mut stream = self.service_client
            .container_client(&self.config.container_name)
            .list_blobs()
            .prefix(prefix.to_string())
            .into_stream();

        let mut names = Vec::new();
        while let Some(response) = futures::StreamExt::next(&mut stream).await {
            let page = response
                .map_err(|e| SidecarError::CloudStorageError(format!("List failed: {}", e)))?;
            for blob in page.blobs.blobs() {
                names.push(blob.name.clone());
            }
        }

        Ok(names)
    }

    pub async fn delete_blob(&self, blob_name: &str) -> Result<(), SidecarError> {
        self.service_client
            .container_client(&self.config.container_name)
            .blob_client(blob_name)
            .delete()
            .await
            .map(|_| ())
            .map_err(|e| SidecarError::CloudStorageError(format!("Delete failed: {}", e)))
    }
}

#[async_trait]
impl CloudConnector for AzureBlobConnector {
    async fn upload(&self, _bucket: &str, key: &str, data: &[u8]) -> Result<String, SidecarError> {
        self.upload_blob(key, data).await
    }

    async fn download(&self, _bucket: &str, key: &str) -> Result<Vec<u8>, SidecarError> {
        self.download_blob(key).await
    }

    async fn list(&self, _bucket: &str, prefix: &str) -> Result<Vec<String>, SidecarError> {
        self.list_blobs(prefix).await
    }

    async fn delete(&self, _bucket: &str, key: &str) -> Result<(), SidecarError> {
        self.delete_blob(key).await
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_azure_blob_config_creation() {
        let config = AzureBlobConfig {
            account_name: "testaccount".to_string(),
            account_key: "dGVzdGtleQ==".to_string(),
            container_name: "testcontainer".to_string(),
        };
        assert_eq!(config.account_name, "testaccount");
        assert_eq!(config.container_name, "testcontainer");
    }

    #[test]
    fn test_azure_blob_connector_creation() {
        let config = AzureBlobConfig {
            account_name: "testaccount".to_string(),
            account_key: "dGVzdGtleQ==".to_string(),
            container_name: "testcontainer".to_string(),
        };
        let result = AzureBlobConnector::new(config);
        assert!(result.is_ok());
    }
}
