# Demo Environment

This document is the operator-facing source of truth for the fixed remote demo environment.

## Goal

Keep one remote environment in a known-good state for live walkthroughs of:

`buyer top-up -> RFQ -> bid -> award -> streaming payout -> completion`

This is not the production launch checklist. It is the shorter contract for demo reliability.

## Fixed actors

- Buyer: configured by `DEMO_BUYER_*`
- Provider: configured by `DEMO_PROVIDER_*`
- Ops: configured by `DEMO_OPS_*`

The org IDs are part of the environment contract. Do not treat them as discoverable runtime output.

## Required services

- `web`
- `api-gateway`
- `iam`
- `marketplace`
- `settlement`
- `settlement-reconciler`
- `execution`
- `carrier-daemon`
- `carrier-gateway`
- `remote-vps`
- `fnn`
- `fnn2`
- `provider-fnn`
- `fiber-adapter`
- `postgres`
- `redis`
- `nats`

## Coolify topology

Deploy the stack from [deploy/coolify/testnet.compose.yaml](../deploy/coolify/testnet.compose.yaml).

Public entrypoints:

- `web`
- `api-gateway`
- `settlement`

Internal-only services:

- `iam`
- `marketplace`
- `settlement-reconciler`
- `execution`
- `risk`
- `notification`
- `carrier-daemon`
- `carrier-gateway`
- `remote-vps`
- `fnn`
- `fnn2`
- `provider-fnn`
- `fiber-adapter`
- `postgres`
- `redis`
- `nats`

## Readiness contract

The demo is only considered ready when:

- the ops control plane reports `ready`
- `/ops` shows `Demo readiness` as `ready`
- buyer prefund is above `DEMO_MIN_BUYER_BALANCE_CENTS`
- provider liquidity is above `DEMO_MIN_PROVIDER_LIQUIDITY_CENTS`
- carrier binding is active
- provider settlement binding is active
- provider pool has at least one ready channel

## Preparation flow

Run this from the deployed environment before a live demo by pressing `Prepare demo` on `/ops`.

The prepare flow will:

- log in or create the fixed buyer, provider, and ops actors
- ensure the provider carrier binding is active
- ensure the provider settlement binding is active
- top up the buyer if the prefund threshold is low
- warm the provider liquidity pool if channel capacity is low
- emit a JSON summary artifact

## Verification flow

Immediately before the live session, refresh `/ops` and confirm `Demo readiness = ready`.

If the control plane remains `blocked`, do not start the live demo.
