CREATE TABLE http_record_kafka
(
    request_headers  Map(String, String),
    request_body     String,
    response_headers Map(String, String),
    response_body    String,
    url              String,
    method           String,
    host             String,
    datetime         DateTime,
    http_version     String
)
ENGINE = Kafka
SETTINGS
    kafka_broker_list = 'kafka:9092',
    kafka_topic_list = 'http-records',
    kafka_group_name = 'clickhouse_http_consumer',
    kafka_format = 'JSONEachRow',
    kafka_num_consumers = 1;




CREATE TABLE http_record_data
(
    request_headers  Map(String, String),
    request_body     String,
    response_headers Map(String, String),
    response_body    String,
    url              String,
    method           String,
    host             String,
    datetime         DateTime,
    http_version     String
)
ENGINE = MergeTree
ORDER BY datetime;



CREATE MATERIALIZED VIEW http_record_mv
TO http_record_data
AS SELECT * FROM http_record_kafka;

