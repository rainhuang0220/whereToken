-- Desensitized MiniMax Agent ledger shape. Tests build a sqlite file from this.
CREATE TABLE local_runtime_token_usage (
  id INTEGER PRIMARY KEY,
  session_id TEXT NOT NULL,
  agent_name TEXT NOT NULL,
  framework_type TEXT NOT NULL,
  turn_id TEXT,
  model TEXT,
  ts INTEGER NOT NULL,
  input_tokens INTEGER NOT NULL,
  output_tokens INTEGER NOT NULL,
  reasoning_tokens INTEGER NOT NULL,
  cache_read_tokens INTEGER NOT NULL,
  cache_write_tokens INTEGER NOT NULL
);
INSERT INTO local_runtime_token_usage
  (id, session_id, agent_name, framework_type, turn_id, model, ts,
   input_tokens, output_tokens, reasoning_tokens, cache_read_tokens, cache_write_tokens)
VALUES
  (1, 's1', 'mavis', 'pi-agent', 'turn-a', 'minimax/MiniMax-M3', 1786267148269, 100, 10, 0, 50, 5),
  (2, 's1', 'mavis', 'pi-agent', 'turn-a', 'minimax/MiniMax-M3', 1786267170642, 20, 8, 2, 80, 0);
