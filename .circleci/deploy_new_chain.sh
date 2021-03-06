#!/bin/bash

echo "Downloading the current testnet Boyar config.json"
curl -O $BOOTSTRAP_URL
echo "Done downloading! Let's begin by cleaning up the testnet of any stale networks for PRs that are already closed"

# Clean up based on PRs which are already closed
node .circleci/testnet/cleanup-mark.js

echo "Copying the newly updated config.json to S3 (with networks to remove)"
aws s3 cp --acl public-read config.json $BOOTSTRAP_S3_URI
echo "Done!"

sleep 60
echo "Verifying the networks are being cleaned.."
node .circleci/testnet/cleanup-poll-disabled-chains.js

echo "Refreshing config.json and removing the dead networks from it.."
rm -f config.json && curl -O $BOOTSTRAP_URL
node .circleci/testnet/cleanup-remove-disabled-chains.js

echo "Copying the newly updated config.json to S3.."
aws s3 cp --acl public-read config.json $BOOTSTRAP_S3_URI
echo "Done!"

echo "Creating a network for this PR within the config.json file.."
PR_CHAIN_ID=$(node .circleci/testnet/add-new-chain.js $CIRCLE_BRANCH $COMMIT_HASH $CI_PULL_REQUESTS)
echo "Done adding a new chain ($PR_CHAIN_ID)"

echo "Copying the newly updated config.json to S3"
aws s3 cp --acl public-read config.json $BOOTSTRAP_S3_URI
echo "Done!"

echo "Configuration updated, waiting for the new PR chain ($PR_CHAIN_ID) to come up!"

echo "Checking deployment status:"
node .circleci/testnet/check-deployment.js $PR_CHAIN_ID
