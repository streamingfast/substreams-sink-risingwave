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


# Alternative: using kafka source

This is pretty similar to using google pubsub:

```sql
CREATE TABLE sol_account_balances (*)
WITH
    (
        connector = 'kafka',
        topic = 'test-topic',
        properties.bootstrap.server = 'localhost:9092'
    ) FORMAT PLAIN ENCODE PROTOBUF (
        message = 'sf.solana.account_sol_balance.v1.Output',
        location = 'https://spkg.io/v1/packages/tl_account_sol_balances_1_0_0/v1.0.0'
    );
```


# Example with nested data

Given this model:
```
struct<
  types_test struct<
    id numeric,
    double_field double precision,
    float_field real,
    int32_field integer,
    int64_field bigint,
    uint32_field bigint,
    uint64_field numeric,
    sint32_field integer,
    sint64_field bigint,
    fixed32_field bigint,
    fixed64_field numeric,
    sfixed32_field integer,
    sfixed64_field bigint,
    bool_field boolean,
    string_field character varying,
    bytes_field bytea,
    timestamp_field struct<
      seconds bigint,
      nanos integer
    >
  >,

  customer struct<
    customer_id character varying,
    name character varying
  >,

  order struct<
    order_id character varying,
    customer_ref_id character varying,
    items struct<
      item_id character varying,
      quantity bigint
    >[]
  >,

  item struct<
    item_id character varying,
    name character varying,
    price double precision
  >
>[]
```

with example data:
```
{
  "@module": "map_output",
  "@block": 313000003,
  "@type": "test.relations.Output",
  "@data": {
    "entities": [
      {
        "typesTest": {
          "id": "313000003",
          "doubleField": 1.7976931348623157e+308,
          "floatField": 3.4028235e+38,
          "int32Field": 2147483647,
          "int64Field": "9223372036854775807",
          "uint32Field": 4294967295,
          "uint64Field": "18446744073709551615",
          "sint32Field": 2147483647,
          "sint64Field": "9223372036854775807",
          "fixed32Field": 4294967295,
          "fixed64Field": "18446744073709551615",
          "sfixed32Field": 2147483647,
          "sfixed64Field": "9223372036854775807",
          "boolField": true,
          "stringField": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
          "bytesField": "AQIDBAUGBwgJCgsMDQ4PEBESExQVFhcYGRobHB0eHyAhIiMkJSYnKCkqKywtLi8wMTIzNDU2Nzg5Ojs8PT4/QEFCQ0RFRkdISUpLTE1OT1BRUlNUVVZXWFlaW1xdXl9gYWJjZGVmZ2hpamtsbW5vcHFyc3R1dnd4eXp7fH1+f4CBgoOEhYaHiImKi4yNjo+QkZKTlJWWl5iZmpucnZ6foKGio6SlpqeoqaqrrK2ur7CxsrO0tba3uLm6u7y9vr/AwcLDxMXGx8jJysvMzc7P0NHS09TV1tfY2drb3N3e3+Dh4uPk5ebn6Onq6+zt7u/w8fLz9PX29/j5+vv8/f7/",
          "timestampField": "1970-01-01T00:00:00Z"
        }
      },
      {
        "customer": {
          "customerId": "customer.id.313000003",
          "name": "customer.name.313000003"
        }
      },
      {
        "item": {
          "itemId": "item.id.313000003",
          "name": "item.name.313000003",
          "price": 99.99
        }
      },
      {
        "order": {
          "orderId": "order.id.313000003",
          "customerRefId": "customer.id.313000003",
          "items": [
            {
              "itemId": "item.id.313000003",
              "quantity": "10"
            }
          ]
        }
      }
    ]
  }
}```

Here is how you can flatten it:
```
CREATE VIEW flat as SELECT unnest(entities) entity from my_table;

CREATE VIEW types_test as SELECT (entity).types_test.* from flat WHERE (entity).types_test.id <> 0;

CREATE VIEW customers as SELECT (entity).customer.* from flat WHERE (entity).customer.customer_id <> '';
CREATE VIEW items as SELECT (entity).item.* from flat WHERE (entity).item.item_id <> '';

CREATE VIEW orders as SELECT (entity).order.order_id, (entity).order.customer_ref_id, UNNEST((entity).order.items) AS item from flat;

CREATE VIEW order_items AS select (entity).order.order_id,  (UNNEST((entity).order.items)).* from flat WHERE (entity).order.order_id <> '';
```


# Next steps

* Figure out how nested data can easily be transformed into tables
* Use a SOURCE instead of a TABLE as the primary data layer (the flattened format would be a better "initial" table or materialized view)
* Manage reorgs / UNDO
