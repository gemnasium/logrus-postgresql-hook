CREATE TABLE logs (
    id SERIAL,
    level smallint NOT NULL,
    message text NOT NULL,
    message_data jsonb NOT NULL,
    created_at timestamp with time zone NOT NULL
);
