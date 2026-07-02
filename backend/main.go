package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	_ "github.com/lib/pq"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

var db *sql.DB

// ========== ESTRUTURAS DE DADOS ==========

type Player struct {
	ID     string
	Conn   *websocket.Conn
	RoomID string
	LastX  float64
	LastY  float64
	LastZ  float64
}

type Mob struct {
	ID       string
	HP       int
	MaxHP    int
	X, Z     float64
	TaggedBy string
	TagTime  time.Time
	RoomID   string
}

type Message struct {
	Type     string  `json:"type"`
	ID       string  `json:"id,omitempty"`
	X        float64 `json:"x"`
	Y        float64 `json:"y"`
	Z        float64 `json:"z"`
	MobID    string  `json:"mob_id,omitempty"`
	HP       int     `json:"hp,omitempty"`
	MaxHP    int     `json:"max_hp,omitempty"`
	TaggedBy string  `json:"tagged_by,omitempty"`
	Item     string  `json:"item,omitempty"`
	Amount   int     `json:"amount,omitempty"`
	RoomID   string  `json:"room_id,omitempty"`
	Level    int     `json:"level,omitempty"`
	XP       int     `json:"xp,omitempty"`
	Gold     int     `json:"gold,omitempty"`
}

// ========== SISTEMA DE ROOMS ==========

type GameRoom struct {
	ID      string
	Players map[string]*Player
	Mobs    map[string]*Mob
	mu      sync.RWMutex
}

var (
	rooms   = make(map[string]*GameRoom)
	roomsMu sync.RWMutex
)

func initRooms() {
	roomIDs := []string{"city", "forest", "mine", "combat1", "combat2"}
	for _, id := range roomIDs {
		rooms[id] = &GameRoom{
			ID:      id,
			Players: make(map[string]*Player),
			Mobs:    make(map[string]*Mob),
		}
	}

	// Spawnar mobs iniciais
	spawnInitialMobs()
}

func spawnInitialMobs() {
	// Floresta: 10 animais passivos (galinhas)
	for i := 0; i < 10; i++ {
		mobID := fmt.Sprintf("chicken_%d", i+1)
		rooms["forest"].Mobs[mobID] = &Mob{
			ID: mobID, HP: 50, MaxHP: 50,
			X: float64(i*3 - 15), Z: float64(i*2 - 10),
			RoomID: "forest",
		}
	}

	// Combate 1: 15 mobs agressivos (lobos)
	for i := 0; i < 15; i++ {
		mobID := fmt.Sprintf("wolf_1_%d", i+1)
		rooms["combat1"].Mobs[mobID] = &Mob{
			ID: mobID, HP: 100, MaxHP: 100,
			X: float64(i*3 - 20), Z: float64(i*2 - 15),
			RoomID: "combat1",
		}
	}

	// Combate 2: 15 mobs agressivos (skeletons)
	for i := 0; i < 15; i++ {
		mobID := fmt.Sprintf("skeleton_2_%d", i+1)
		rooms["combat2"].Mobs[mobID] = &Mob{
			ID: mobID, HP: 150, MaxHP: 150,
			X: float64(i*3 - 20), Z: float64(i*2 - 15),
			RoomID: "combat2",
		}
	}

	log.Printf("✅ Mobs spawnados: %d na floresta, %d no combate1, %d no combate2",
		len(rooms["forest"].Mobs), len(rooms["combat1"].Mobs), len(rooms["combat2"].Mobs))
}

// ========== SISTEMA DE SEASONS ==========

var seasonTicker *time.Ticker
var currentSeason int = 1

func startSeasonSystem() {
	// Para teste: reset a cada 1 minuto (mude para 7 * 24 * time.Hour em produção)
	seasonTicker = time.NewTicker(1 * time.Minute)
	go func() {
		for range seasonTicker.C {
			resetSeason()
		}
	}()
	log.Println("✅ Sistema de seasons iniciado (reset a cada 1 minuto para teste)")
}

func resetSeason() {
	log.Printf("🔄 Resetando Season %d...", currentSeason)

	// 1. Salvar rankings da season
	saveSeasonRankings()

	// 2. Resetar players (soft reset: manter NFTs)
	resetPlayerProgress()

	// 3. Incrementar season
	currentSeason++

	// 4. Notificar todos os players
	broadcastSeasonReset()

	log.Printf("✅ Season %d iniciada!", currentSeason)
}

func saveSeasonRankings() {
	log.Println("📊 Salvando rankings da season...")
	// Aqui você query o banco e salva os top 100
	// Por enquanto, só log
}

func resetPlayerProgress() {
	log.Println("🔄 Resetando progressão dos players...")
	// UPDATE players SET level = 1, xp = 0, gold = 0, inventory = '{}' WHERE reset_on_season = true
}

func broadcastSeasonReset() {
	msg := Message{Type: "season_reset", Level: currentSeason}
	data, _ := json.Marshal(msg)

	roomsMu.RLock()
	defer roomsMu.RUnlock()

	for _, room := range rooms {
		room.mu.RLock()
		for _, player := range room.Players {
			player.Conn.WriteMessage(websocket.TextMessage, data)
		}
		room.mu.RUnlock()
	}
}

// ========== PERSISTÊNCIA (POSTGRESQL) ==========

func initDB() {
	var err error
	db, err = sql.Open("postgres", "user=gameuser password=gamepass dbname=gamedb sslmode=disable")
	if err != nil {
		log.Fatal("❌ Erro ao conectar no PostgreSQL:", err)
	}

	// Criar player se não existir
	_, err = db.Exec(`
		INSERT INTO players (id, level, experience, gold, inventory)
		VALUES ($1, 1, 0, 0, '{}')
		ON CONFLICT (id) DO NOTHING
	`, "test_player")

	if err != nil {
		log.Println("⚠️ Erro ao criar player de teste:", err)
	}

	log.Println("✅ PostgreSQL conectado!")
}

func savePlayerData(playerID string, level, xp, gold int) {
	_, err := db.Exec(`
		UPDATE players 
		SET level = $1, experience = $2, gold = $3, last_login = NOW()
		WHERE id = $4
	`, level, xp, gold, playerID)

	if err != nil {
		log.Println("❌ Erro ao salvar player:", err)
	}
}

func giveXP(playerID string, amount int) {
	// Query atual: level, xp
	var level, xp int
	err := db.QueryRow("SELECT level, experience FROM players WHERE id = $1", playerID).Scan(&level, &xp)
	if err != nil {
		log.Println("❌ Erro ao buscar player:", err)
		return
	}

	xp += amount

	// Check level up (100 XP por level)
	for xp >= level*100 {
		xp -= level * 100
		level++
		log.Printf("🎉 Player %s subiu para level %d!", playerID, level)
	}

	savePlayerData(playerID, level, xp, 0)
}

func giveGold(playerID string, amount int) {
	_, err := db.Exec(`
		UPDATE players 
		SET gold = gold + $1 
		WHERE id = $2
	`, amount, playerID)

	if err != nil {
		log.Println("❌ Erro ao dar gold:", err)
	}
}

func giveLoot(playerID string, item string, amount int) {
	_, err := db.Exec(`
		UPDATE players 
		SET inventory = jsonb_set(
			inventory, 
			ARRAY[$1], 
			to_jsonb(COALESCE((inventory->>$1)::int, 0) + $2)
		)
		WHERE id = $3
	`, item, amount, playerID)

	if err != nil {
		log.Println("❌ Erro ao dar loot:", err)
	}
}

// ========== WEBSOCKET HANDLER ==========

func handleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println("Erro no upgrade:", err)
		return
	}
	defer conn.Close()

	var player *Player

	for {
		_, msgBytes, err := conn.ReadMessage()
		if err != nil {
			if player != nil {
				removePlayerFromRoom(player)
				broadcastPlayerDisconnect(player.ID, player.RoomID)
				log.Printf("❌ Player %s desconectado.", player.ID)
			}
			break
		}

		var msg Message
		if err := json.Unmarshal(msgBytes, &msg); err != nil {
			continue
		}

		// Primeira mensagem: registrar player
		if player == nil && msg.ID != "" {
			player = &Player{
				ID:    msg.ID,
				Conn:  conn,
				LastX: msg.X, // Garante que a posição inicial não seja 0,0,0
				LastY: msg.Y,
				LastZ: msg.Z,
			}

			addPlayerToRoom(player, "city")

			log.Printf("✅ Player %s ADICIONADO na city! Total na room: %d", player.ID, len(rooms["city"].Players))

			// Envia mobs e players existentes para o NOVO player
			sendInitialState(player)

			// ✅ CORREÇÃO CRÍTICA: Avisar os players EXISTENTES que o NOVO player chegou
			broadcastPlayerSpawn(player)
		}

		if player == nil {
			continue
		}

		switch msg.Type {
		case "move":
			handleMove(player, msg)
		case "attack":
			handleAttack(player, msg)
		case "change_room":
			handleRoomChange(player, msg)
		}
	}
}

func addPlayerToRoom(player *Player, roomID string) {
	roomsMu.RLock()
	room, exists := rooms[roomID]
	roomsMu.RUnlock()

	if !exists {
		log.Printf("❌ Room %s não existe!", roomID)
		return
	}

	room.mu.Lock()
	room.Players[player.ID] = player
	player.RoomID = roomID
	room.mu.Unlock()
}

func removePlayerFromRoom(player *Player) {
	roomsMu.RLock()
	room, exists := rooms[player.RoomID]
	roomsMu.RUnlock()

	if !exists {
		return
	}

	room.mu.Lock()
	delete(room.Players, player.ID)
	room.mu.Unlock()
}

func sendInitialState(player *Player) {
	roomsMu.RLock()
	room := rooms[player.RoomID]
	roomsMu.RUnlock()

	room.mu.RLock()
	defer room.mu.RUnlock()

	// Enviar mobs da room
	for _, mob := range room.Mobs {
		if mob.HP > 0 {
			msg := Message{
				Type: "mob_spawn", MobID: mob.ID,
				X: mob.X, Z: mob.Z, HP: mob.HP, MaxHP: mob.MaxHP,
			}
			data, _ := json.Marshal(msg)
			player.Conn.WriteMessage(websocket.TextMessage, data)
		}
	}

	// Enviar outros players da room
	for id, otherPlayer := range room.Players {
		if id != player.ID {
			msg := Message{
				Type: "player_spawn", ID: id,
				X: otherPlayer.LastX, Y: otherPlayer.LastY, Z: otherPlayer.LastZ,
			}
			data, _ := json.Marshal(msg)
			player.Conn.WriteMessage(websocket.TextMessage, data)
		}
	}
}

func handleMove(player *Player, msg Message) {
	player.LastX = msg.X
	player.LastY = msg.Y
	player.LastZ = msg.Z

	roomsMu.RLock()
	room := rooms[player.RoomID]
	roomsMu.RUnlock()

	room.mu.RLock()
	defer room.mu.RUnlock()

	// Broadcast para outros players da mesma room
	msg.ID = player.ID
	data, _ := json.Marshal(msg)

	for id, otherPlayer := range room.Players {
		if id != player.ID {
			otherPlayer.Conn.WriteMessage(websocket.TextMessage, data)
		}
	}
}

func handleAttack(player *Player, msg Message) {
	roomsMu.RLock()
	room := rooms[player.RoomID]
	roomsMu.RUnlock()

	room.mu.Lock()
	defer room.mu.Unlock()

	mob, exists := room.Mobs[msg.MobID]
	if !exists || mob.HP <= 0 {
		return
	}

	// Sistema de tagging
	tagExpired := time.Since(mob.TagTime) > 10*time.Second
	if mob.TaggedBy == "" || tagExpired {
		mob.TaggedBy = player.ID
		mob.TagTime = time.Now()
		log.Printf("🏷️ Mob %s tagueado por %s", mob.ID, player.ID)
	}

	// Aplicar dano
	damage := 20
	mob.HP -= damage
	if mob.HP < 0 {
		mob.HP = 0
	}

	// Renovar tag se o dono atacou
	if mob.TaggedBy == player.ID {
		mob.TagTime = time.Now()
	}

	// Broadcast hit
	hitMsg := Message{
		Type: "mob_hit", MobID: mob.ID, HP: mob.HP, MaxHP: mob.MaxHP,
		TaggedBy: mob.TaggedBy, ID: player.ID,
	}
	hitData, _ := json.Marshal(hitMsg)

	for _, p := range room.Players {
		p.Conn.WriteMessage(websocket.TextMessage, hitData)
	}

	// Mob morreu
	if mob.HP <= 0 {
		log.Printf("💀 Mob %s morreu! Killer: %s", mob.ID, mob.TaggedBy)

		// Broadcast morte
		deadMsg := Message{Type: "mob_dead", MobID: mob.ID}
		deadData, _ := json.Marshal(deadMsg)
		for _, p := range room.Players {
			p.Conn.WriteMessage(websocket.TextMessage, deadData)
		}

		// Dar recompensas para o killer
		if mob.TaggedBy != "" {
			giveXP(mob.TaggedBy, 50)
			giveGold(mob.TaggedBy, 10)
			giveLoot(mob.TaggedBy, "Feather", 1)

			// Enviar loot para o killer
			lootMsg := Message{Type: "loot", Item: "Feather", Amount: 1, ID: mob.TaggedBy}
			lootData, _ := json.Marshal(lootMsg)
			if killer, ok := room.Players[mob.TaggedBy]; ok {
				killer.Conn.WriteMessage(websocket.TextMessage, lootData)
			}
		}

		// Respawnar mob depois de 30s
		go func(mobID, roomID string) {
			time.Sleep(30 * time.Second)

			roomsMu.RLock()
			r := rooms[roomID]
			roomsMu.RUnlock()

			r.mu.Lock()
			if m, ok := r.Mobs[mobID]; ok {
				m.HP = m.MaxHP
				m.TaggedBy = ""

				spawnMsg := Message{
					Type: "mob_spawn", MobID: m.ID,
					X: m.X, Z: m.Z, HP: m.HP, MaxHP: m.MaxHP,
				}
				spawnData, _ := json.Marshal(spawnMsg)

				for _, p := range r.Players {
					p.Conn.WriteMessage(websocket.TextMessage, spawnData)
				}
			}
			r.mu.Unlock()
		}(mob.ID, player.RoomID)
	}
}

func handleRoomChange(player *Player, msg Message) {
	newRoomID := msg.RoomID

	roomsMu.RLock()
	_, exists := rooms[newRoomID]
	roomsMu.RUnlock()

	if !exists {
		log.Printf("❌ Room %s não existe!", newRoomID)
		return
	}

	// Remover da room atual
	oldRoomID := player.RoomID
	removePlayerFromRoom(player)

	// Broadcast desconexão da room antiga
	broadcastPlayerDisconnect(player.ID, oldRoomID)

	// Adicionar na nova room
	addPlayerToRoom(player, newRoomID)

	// Enviar estado da nova room
	sendInitialState(player)

	// Broadcast para a nova room
	broadcastPlayerSpawn(player)

	log.Printf("🚪 Player %s mudou de %s para %s", player.ID, oldRoomID, newRoomID)
}

func broadcastPlayerDisconnect(playerID, roomID string) {
	roomsMu.RLock()
	room, exists := rooms[roomID]
	roomsMu.RUnlock()

	if !exists {
		return
	}

	room.mu.RLock()
	defer room.mu.RUnlock()

	msg := Message{Type: "player_disconnected", ID: playerID}
	data, _ := json.Marshal(msg)

	for _, p := range room.Players {
		p.Conn.WriteMessage(websocket.TextMessage, data)
	}
}

func broadcastPlayerSpawn(player *Player) {
	roomsMu.RLock()
	room, exists := rooms[player.RoomID]
	roomsMu.RUnlock()

	if !exists {
		log.Printf("⚠️ broadcastPlayerSpawn: Room %s não existe!", player.RoomID)
		return
	}

	room.mu.RLock()
	defer room.mu.RUnlock()

	log.Printf("📢 Broadcastando spawn de %s para %d players na room %s", player.ID, len(room.Players)-1, player.RoomID)

	msg := Message{
		Type: "player_spawn", ID: player.ID,
		X: player.LastX, Y: player.LastY, Z: player.LastZ,
	}
	data, _ := json.Marshal(msg)

	for id, p := range room.Players {
		if id != player.ID {
			log.Printf("   -> Enviando player_spawn de %s para %s", player.ID, id)
			p.Conn.WriteMessage(websocket.TextMessage, data)
		}
	}
}

// ========== MAIN ==========

func main() {
	// Inicializar banco de dados
	initDB()

	// Inicializar rooms
	initRooms()

	// Iniciar sistema de seasons
	startSeasonSystem()

	// WebSocket endpoint
	http.HandleFunc("/ws", handleWebSocket)

	port := "8080"
	fmt.Printf("🚀 Servidor Go rodando em ws://localhost:%s/ws\n", port)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}
