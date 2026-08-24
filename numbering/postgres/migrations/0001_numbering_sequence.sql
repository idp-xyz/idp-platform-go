-- 0001_numbering_sequence：按键原子单据编号序列。
-- 本资产不可变（只增不改）；执行、权限与历史归消费者。
CREATE SCHEMA IF NOT EXISTS {{schema}};

CREATE TABLE {{schema}}.numbering_sequence (
    scope      text        NOT NULL,
    doc_type   text        NOT NULL,
    period     text        NOT NULL DEFAULT '',
    next_val   bigint      NOT NULL,
    updated_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (scope, doc_type, period)
);
