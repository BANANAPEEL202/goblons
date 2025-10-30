# Goblons.io Deployment Guide

This guide covers deploying Goblons.io to Azure Container Instances using two methods:

## Method 1: Automated GitHub Actions (Recommended)

### Prerequisites
1. Azure account with active subscription
2. GitHub repository with Actions enabled

### Setup Steps

1. **Create Azure Service Principal**
   ```bash
   # Login to Azure
   az login

   # Create service principal (replace YOUR_SUBSCRIPTION_ID)
   az ad sp create-for-rbac \
     --name "goblons-deploy" \
     --role contributor \
     --scopes /subscriptions/YOUR_SUBSCRIPTION_ID \
     --sdk-auth
   ```

2. **Add GitHub Secret**
   - Go to your GitHub repository settings
   - Navigate to Secrets and Variables → Actions
   - Create a new repository secret named `AZURE_CREDENTIALS`
   - Paste the entire JSON output from the previous command

3. **Trigger Deployment**
   - Push to `main`, `master`, or `animations` branch
   - Or manually trigger via Actions tab → "Deploy Goblons.io to Azure" → Run workflow

4. **Access Your Game**
   - After deployment completes, check the Actions log for the game URL
   - Usually: `http://goblons-ci.eastus.azurecontainer.io:8080`

## Method 2: Manual Deployment Script

If you prefer manual deployment or the GitHub Actions approach doesn't work:

1. **Make script executable**
   ```bash
   chmod +x deploy-azure.sh
   ```

2. **Login to Azure**
   ```bash
   az login
   ```

3. **Run deployment**
   ```bash
   ./deploy-azure.sh
   ```

## Troubleshooting

### Common Issues

1. **"Resource group not found"**
   - The script automatically creates the resource group
   - Ensure you have proper permissions in your Azure subscription

2. **"Registry name already taken"**
   - Azure Container Registry names must be globally unique
   - Edit the registry name in either `deploy-azure.sh` or `.github/workflows/deploy.yml`

3. **Container won't start**
   - Check Azure Portal → Container Instances → your container → Logs
   - Common issue: Port 8080 not exposed in Dockerfile (already configured)

4. **GitHub Actions fails with permissions**
   - Verify the service principal JSON is correctly copied to GitHub secrets
   - Check that the service principal has Contributor role on your subscription

### Monitoring

- **View logs**: Azure Portal → Container Instances → goblons-game → Logs
- **Check status**: Azure Portal → Container Instances → goblons-game → Overview
- **Monitor costs**: Azure Portal → Cost Management

### Updating the Game

**GitHub Actions**: Simply push code changes to main/master/animations branch

**Manual**: Run `./deploy-azure.sh` again after making changes

## Architecture

- **Container Registry**: Stores your Docker images
- **Container Instances**: Runs your game server
- **Resource Group**: Contains all related resources
- **DNS**: Provides a friendly URL for your game

## Cost Estimation

With the default configuration (0.5 CPU, 1GB RAM):
- **Container Instances**: ~$13/month (if running 24/7)
- **Container Registry**: ~$5/month
- **Total**: ~$18/month

To reduce costs:
- Stop the container when not in use: `az container stop -g goblons-rg -n goblons-game`
- Use smaller CPU/memory if performance allows

## Next Steps

1. **Custom Domain**: Configure a custom domain in Azure DNS
2. **SSL/HTTPS**: Add Application Gateway for SSL termination
3. **Scaling**: Move to Azure Container Apps for auto-scaling
4. **Monitoring**: Add Application Insights for detailed monitoring