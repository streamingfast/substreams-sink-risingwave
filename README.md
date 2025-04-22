# Bootstrap dev environment

## 1. Pubsub

In a terminal:

* Install gcloud: https://cloud.google.com/sdk/docs/install-sdk
* Install pubsub emulator: gcloud components install pubsub-emulator && gcloud components update
* Run pubsub emulator : `gcloud beta emulators pubsub start --project=dev --host-port=0.0.0.0:8085`

In another terminal:

* Create a 'test' topic: `http PUT http://localhost:8085/v1/projects/dev/topics/test`
* Create a 'test' subscription: `echo {\'topic\':\'projects/dev/topics/test\'} | http PUT http://localhost:8085/v1/projects/dev/subscriptions/test`

## 2. Risingwave

In a terminal:

* Install risingwave: https://docs.risingwave.com/get-started/quickstart
* Run risingwave: `risingwave`

In another terminal:

* Install PSQL: `brew install libpq && brew link --force libpq`
* Connect to risingwave using psql in yet another terminal: `psql -h 0.0.0.0 -p 4566 -d dev -U root` (deps: )

## 3. Create your table/substreams_source in Risingwave

* Choose your 'spkg' file (ex: from https://substreams.dev), it contains the protobuf definition of the data you want to stream. (ex: https://spkg.io/v1/packages/tl_account_sol_balances_1_0_0/v1.0.0)
* Identify the output module that you want and its protobuf output type (ex: `map_block` -> `sf.substreams.v1.Transactions`)
* Create the table that will read from the pubsub

```sql
CREATE TABLE sol_account_balances (
    *,
)
WITH
    (
        connector = 'google_pubsub',
        pubsub.subscription = 'projects/dev/subscriptions/test',
        pubsub.emulator_host = 'localhost:8085'
    ) FORMAT PLAIN ENCODE PROTOBUF (
        message = 'sf.solana.account_sol_balance.v1.Output',
        schema.location = 'https://spkg.io/v1/packages/tl_account_sol_balances_1_0_0/v1.0.0'
    );
```

## 4. Launch this substreams sink

In another terminal:

* Get the environment variables for the pubsub simulator: ` $(gcloud beta emulators pubsub env-init)`
* Run the sink: `go run ./cmd/substreams-sink-risingwave mainnet.sol.streamingfast.io:443 https://spkg.io/v1/packages/tl_account_sol_balances_1_0_0/v1.0.0 map_block 313000000:313000010 --project dev --topic test`

## 5. Query your data

In the `psql` terminal:

* Query the data: `SELECT * FROM sol_account_balances;`

* Observe that the data is an array of `sf.solana.account_sol_balance.v1.Output.data` objects.

* Create a view with the flattened data:

```sql
CREATE VIEW sol_account_balances_flat AS
SELECT
    (data).block_slot,
    (data).block_date,
    (data).account,
    (data).post_balance
FROM
    (
        select
            unnest (data) data
        from
            sol_account_balances
    );
```

* You can create a materialized view with aggregations from that data:
```sql
CREATE MATERIALIZED VIEW sol_balances_view AS
SELECT
    SUM(post_balance) AS total_balance,
    AVG(post_balance) AS average_balance,
    COUNT(account) AS accounts
FROM
    sol_account_balances_flat
GROUP BY
    block_slot;
```

* Note that it automatically updates the materialized view when new data is inserted into the view, by running the sink again on another block range...

## 6. Resetting the environment:

1. CTRL-C on Risingwave
2. CTRL-C on pubsub emulator
3. `rm -rf ~/.risingwave`
4. Restart pubsub emulator `gcloud beta emulators pubsub start --project=dev --host-port=0.0.0.0:8085`
5. Create the topic and subscriptions again: `http PUT http://localhost:8085/v1/projects/dev/topics/test && echo {\'topic\':\'projects/dev/topics/test\'} | http PUT http://localhost:8085/v1/projects/dev/subscriptions/test`
6. Restart RisingWave `risingwave`

# Next steps

* Figure out how nested data can easily be transformed into tables
* Use a SOURCE instead of a TABLE as the primary data layer (the flattened format would be a better "initial" table or materialized view)
* Manage reorgs / UNDO
