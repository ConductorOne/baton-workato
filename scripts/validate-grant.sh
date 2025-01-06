#!/usr/bin/env bash
set -exo pipefail

BATON_PRINCIPAL=$1
BATON_PRINCIPAL_TYPE=$2
BATON_ENTITLEMENT=$3
BATON_GRANT=$4

if [ -z "$BATON_CONNECTOR" ]; then
  echo "BATON_CONNECTOR not set."
  exit
fi
if [ -z "$BATON" ]; then
  echo "BATON not set. using baton"
  BATON=baton
fi
# Error on unbound variables now that we've set BATON & BATON_CONNECTOR
set -u

# SYNC
$BATON_CONNECTOR

# GRANT
$BATON_CONNECTOR --grant-entitlement="$BATON_ENTITLEMENT" --grant-principal="$BATON_PRINCIPAL" --grant-principal-type="$BATON_PRINCIPAL_TYPE"

# SYNC
$BATON_CONNECTOR
# Check grant was granted
$BATON grants --entitlement="$BATON_ENTITLEMENT" --output-format=json | jq --exit-status ".grants[] | select( .principal.id.resource == \"$BATON_PRINCIPAL\" )"

## Revoke grant
$BATON_CONNECTOR --revoke-grant="$BATON_GRANT"
# Revoke already-revoked grant
$BATON_CONNECTOR --revoke-grant="$BATON_GRANT"

# Check grant was revoked
$BATON_CONNECTOR
$BATON grants --entitlement="$BATON_ENTITLEMENT" --output-format=json | jq --exit-status "if .grants then [ .grants[] | select( .principal.id.resource == \"$BATON_PRINCIPAL\" ) ] | length == 0 else . end"

# Re-grant entitlement
$BATON_CONNECTOR --grant-entitlement="$BATON_ENTITLEMENT" --grant-principal="$BATON_PRINCIPAL" --grant-principal-type="$BATON_PRINCIPAL_TYPE"

# Check grant was re-granted
$BATON_CONNECTOR
$BATON grants --entitlement="$BATON_ENTITLEMENT" --output-format=json | jq --exit-status ".grants[] | select( .principal.id.resource == \"$BATON_PRINCIPAL\" )"

## Revoke grant
$BATON_CONNECTOR --revoke-grant="$BATON_GRANT"