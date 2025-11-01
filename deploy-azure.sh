#!/bin/bash

# Manual Azure Deployment Script for Goblons.io
# Run this script to deploy your game manually to Azure Container Instances

# Configuration - UPDATE THESE VALUES BEFORE RUNNING
RESOURCE_GROUP="goblons-rg"
LOCATION="eastus"
CONTAINER_NAME="goblons-game"
IMAGE_NAME="goblons"
REGISTRY_NAME="goblonsregistry$(whoami)"  # Adds your username for uniqueness
DNS_LABEL="goblons-$(whoami)"             # Adds your username for uniqueness

echo "🚀 Manual deployment of Goblons.io to Azure..."
echo "📋 Using configuration:"
echo "   Resource Group: $RESOURCE_GROUP"
echo "   Registry: $REGISTRY_NAME"
echo "   DNS Label: $DNS_LABEL"
echo ""

# Check if Azure CLI is installed and logged in
if ! command -v az &> /dev/null; then
    echo "❌ Azure CLI is not installed. Please install it first:"
    echo "   brew install azure-cli"
    exit 1
fi

# Check if logged in to Azure
if ! az account show &> /dev/null; then
    echo "❌ Not logged in to Azure. Please run: az login"
    exit 1
fi

echo "✅ Azure CLI is ready"

# Step 1: Register required resource providers
echo "🔧 Registering Azure resource providers..."
echo "   This may take a few minutes on first run..."
az provider register --namespace Microsoft.ContainerRegistry --wait
az provider register --namespace Microsoft.ContainerInstance --wait
echo "✅ Resource providers registered"

# Step 2: Create resource group
echo "📁 Creating resource group..."
az group create \
  --name $RESOURCE_GROUP \
  --location $LOCATION \
  --output table

# Step 3: Create Azure Container Registry (ACR)
echo "🐳 Creating container registry..."
az acr create \
  --resource-group $RESOURCE_GROUP \
  --name $REGISTRY_NAME \
  --sku Basic \
  --location $LOCATION \
  --output table

# Enable admin access for the registry
echo "🔧 Enabling admin access on container registry..."
az acr update \
  --name $REGISTRY_NAME \
  --admin-enabled true \
  --output table

echo "⏳ Waiting for registry to be ready..."
sleep 10

# Step 4: Build and push Docker image
echo "🔨 Building and pushing Docker image..."
echo "   This may take a few minutes..."
az acr build \
  --registry $REGISTRY_NAME \
  --image $IMAGE_NAME:latest \
  --file Dockerfile \
  . \
  --output table

# Step 5: Deploy to Azure Container Instances
echo "☁️ Deploying to Azure Container Instances..."

# Get ACR credentials for authentication
REGISTRY_USERNAME=$(az acr credential show --name $REGISTRY_NAME --query username -o tsv)
REGISTRY_PASSWORD=$(az acr credential show --name $REGISTRY_NAME --query passwords[0].value -o tsv)

az container create \
  --resource-group $RESOURCE_GROUP \
  --name $CONTAINER_NAME \
  --image $REGISTRY_NAME.azurecr.io/$IMAGE_NAME:latest \
  --registry-login-server $REGISTRY_NAME.azurecr.io \
  --registry-username $REGISTRY_USERNAME \
  --registry-password $REGISTRY_PASSWORD \
  --dns-name-label $DNS_LABEL \
  --ports 8080 \
  --cpu 2.0 \
  --memory 2 \
  --os-type Linux \
  --location $LOCATION \
  --output table

# Wait for container to start
echo "⏳ Waiting for container to start..."
sleep 30

# Step 6: Show results
echo ""
echo "🎉 Deployment complete!"
echo ""
echo "📍 Your game URLs:"
echo "   Public URL: http://$DNS_LABEL.$LOCATION.azurecontainer.io:8080"
echo ""
echo "🔍 Useful commands:"
echo "   Check status: az container show --resource-group $RESOURCE_GROUP --name $CONTAINER_NAME --query instanceView.state"
echo "   View logs:    az container logs --resource-group $RESOURCE_GROUP --name $CONTAINER_NAME"
echo "   Restart:      az container restart --resource-group $RESOURCE_GROUP --name $CONTAINER_NAME"
echo "   Delete:       az container delete --resource-group $RESOURCE_GROUP --name $CONTAINER_NAME --yes"
echo ""

# Show current status
echo "📊 Current container status:"
az container show \
  --resource-group $RESOURCE_GROUP \
  --name $CONTAINER_NAME \
  --query "{State:instanceView.state,IP:ipAddress.ip,FQDN:ipAddress.fqdn}" \
  --output table