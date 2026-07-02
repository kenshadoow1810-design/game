-- Tabela de Players
CREATE TABLE players (
    id VARCHAR(255) PRIMARY KEY,
    wallet_address VARCHAR(255) UNIQUE,
    level INT DEFAULT 1,
    experience INT DEFAULT 0,
    gold INT DEFAULT 0,
    inventory JSONB DEFAULT '{}',
    equipment JSONB DEFAULT '{}',
    nfts JSONB DEFAULT '[]',
    achievements JSONB DEFAULT '[]',
    current_room VARCHAR(50) DEFAULT 'city',
    season_score INT DEFAULT 0,
    created_at TIMESTAMP DEFAULT NOW(),
    last_login TIMESTAMP DEFAULT NOW()
);

-- Tabela de Seasons
CREATE TABLE seasons (
    id SERIAL PRIMARY KEY,
    season_number INT UNIQUE NOT NULL,
    start_date TIMESTAMP DEFAULT NOW(),
    end_date TIMESTAMP,
    status VARCHAR(20) DEFAULT 'active',
    created_at TIMESTAMP DEFAULT NOW()
);

-- Tabela de Rankings de Season
CREATE TABLE season_rankings (
    id SERIAL PRIMARY KEY,
    season_number INT REFERENCES seasons(season_number),
    player_id VARCHAR(255) REFERENCES players(id),
    final_level INT,
    final_score INT,
    rank_position INT,
    rewards_claimed BOOLEAN DEFAULT false,
    created_at TIMESTAMP DEFAULT NOW(),
    UNIQUE(season_number, player_id)
);

-- Tabela de NFTs (itens persistentes)
CREATE TABLE nfts (
    id SERIAL PRIMARY KEY,
    mint_address VARCHAR(255) UNIQUE,
    player_id VARCHAR(255) REFERENCES players(id),
    item_type VARCHAR(50),
    item_name VARCHAR(255),
    item_rarity VARCHAR(20),
    season_number INT,
    metadata JSONB DEFAULT '{}',
    created_at TIMESTAMP DEFAULT NOW()
);

-- Tabela de Transações
CREATE TABLE transactions (
    id SERIAL PRIMARY KEY,
    player_id VARCHAR(255) REFERENCES players(id),
    transaction_type VARCHAR(50),
    item_id INT REFERENCES nfts(id),
    amount INT,
    counterparty_id VARCHAR(255),
    solana_tx_hash VARCHAR(255),
    created_at TIMESTAMP DEFAULT NOW()
);

-- Tabela de Estado do Jogo
CREATE TABLE game_state (
    id SERIAL PRIMARY KEY,
    current_season_number INT DEFAULT 1,
    season_start_date TIMESTAMP DEFAULT NOW(),
    season_end_date TIMESTAMP,
    total_players INT DEFAULT 0,
    total_transactions INT DEFAULT 0
);

-- Inserir estado inicial
INSERT INTO game_state (current_season_number) VALUES (1);
INSERT INTO seasons (season_number) VALUES (1);

-- Índices para performance
CREATE INDEX idx_players_level ON players(level DESC);
CREATE INDEX idx_players_season_score ON players(season_score DESC);
CREATE INDEX idx_season_rankings_season ON season_rankings(season_number, rank_position);
CREATE INDEX idx_nfts_player ON nfts(player_id);
CREATE INDEX idx_transactions_player ON transactions(player_id);