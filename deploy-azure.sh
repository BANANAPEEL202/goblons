#!/bin/bash

# ============================================
# Manual Azure Deployment Script for Goblons.io
# Deploys a Dockerized game server on a low-cost Azure VM
# ============================================

set -e

# =========================
# Configuration
# =========================
RESOURCE_GROUP="goblons-rg"
LOCATION="eastus"
VM_NAME="goblons-vm"
VM_SIZE="Standard_B2s"           # 2 vCPU, 4 GB RAM (low-cost)
IMAGE_NAME="goblons"
REGISTRY_NAME="goblonsregistry$(whoami)"  # Unique name per user
ADMIN_USER="azureuser"
SSH_KEY="$HOME/.ssh/id_rsa.pub"

# =========================
# Helper Functions
# =========================
check_azure_cli() {
    if ! command -v az &> /dev/null; then
        echo "❌ Azure CLI not installed. Install it first: brew install azure-cli"
        exit 1
    fi
    if ! az account show &> /dev/null; then
        echo "❌ Not logged in. Run: az login"
        exit 1
    fi
    echo "✅ Azure CLI ready"
}

# =========================
# 1. Prepare Azure Resources
# =========================
check_azure_cli

echo "🔧 Registering resource providers..."
az provider register --namespace Microsoft.ContainerRegistry --wait
az provider register --namespace Microsoft.Compute --wait
az provider register --namespace Microsoft.Network --wait
echo "✅ Resource providers registered"

echo "📁 Creating resource group..."
az group create --name "$RESOURCE_GROUP" --location "$LOCATION" --output table

# =========================
# 2. Create ACR
# =========================
echo "🐳 Creating container registry..."
az acr create \
    --resource-group "$RESOURCE_GROUP" \
    --name "$REGISTRY_NAME" \
    --sku Basic \
    --location "$LOCATION" \
    --output table

echo "🔧 Enabling admin access..."
az acr update --name "$REGISTRY_NAME" --admin-enabled true --output table
sleep 10

# =========================
# 3. Build and Push Image
# =========================
echo "🔨 Building and pushing Docker image..."
az acr build \
    --registry "$REGISTRY_NAME" \
    --image "$IMAGE_NAME:latest" \
    --file Dockerfile \
    . \
    --output table

# =========================
# 4. Create VM
# =========================
echo "💻 Creating Azure VM ($VM_SIZE)..."
az vm create \
  --resource-group "$RESOURCE_GROUP" \
  --name "$VM_NAME" \
  --image Ubuntu2204 \
  --size "$VM_SIZE" \
  --admin-username "$ADMIN_USER" \
  --ssh-key-values "$SSH_KEY" \
  --public-ip-sku Standard \
  --output table

# Open port 8080 for game traffic
echo "🌐 Opening port 8080..."
az vm open-port \
  --port 8080 \
  --resource-group "$RESOURCE_GROUP" \
  --name "$VM_NAME"

# =========================
# 5. Connect to VM and Run Container
# =========================
echo "⏳ Waiting for VM public IP..."
sleep 30
VM_IP=$(az vm show --resource-group "$RESOURCE_GROUP" --name "$VM_NAME" --show-details --query publicIps -o tsv)

REGISTRY_USERNAME=$(az acr credential show --name "$REGISTRY_NAME" --query username -o tsv)
REGISTRY_PASSWORD=$(az acr credential show --name "$REGISTRY_NAME" --query passwords[0].value -o tsv)

echo "🔗 Connecting to VM ($VM_IP) and setting up container..."
ssh -o StrictHostKeyChecking=no "$ADMIN_USER@$VM_IP" << EOF
    set -e
    echo "🧰 Installing Docker..."
    sudo apt-get update -y
    sudo apt-get install -y docker.io
    sudo systemctl enable --now docker

    echo "🔐 Logging into ACR..."
    echo "$REGISTRY_PASSWORD" | sudo docker login "$REGISTRY_NAME.azurecr.io" -u "$REGISTRY_USERNAME" --password-stdin

    echo "🐳 Pulling latest image..."
    sudo docker pull "$REGISTRY_NAME.azurecr.io/$IMAGE_NAME:latest"

    echo "🧹 Cleaning old container..."
    sudo docker rm -f "$IMAGE_NAME" 2>/dev/null || true

    echo "🚀 Starting game container..."
    sudo docker run -d -p 8080:8080 --restart unless-stopped --name "$IMAGE_NAME" "$REGISTRY_NAME.azurecr.io/$IMAGE_NAME:latest"
EOF

# =========================
# 6. Done
# =========================
echo ""
echo "🎉 Deployment complete!"
echo "📍 Game running at: http://$VM_IP:8080"
echo ""
echo "🔍 SSH into VM: az vm ssh --name $VM_NAME --resource-group $RESOURCE_GROUP"
echo "   Docker commands inside VM:"
echo "     docker logs $IMAGE_NAME"
echo "     docker restart $IMAGE_NAME"
echo "     docker stop $IMAGE_NAME"
echo "     docker rm $IMAGE_NAME"
echo ""
