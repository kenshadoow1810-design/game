var NetworkManager = pc.createScript('networkManager');

NetworkManager.attributes.add('wsUrl', { type: 'string', default: 'ws://localhost:8080/ws' });
NetworkManager.attributes.add('localPlayerName', { type: 'string', default: 'Player' });

NetworkManager.prototype.initialize = function() {
    this.localPlayer = this.app.root.findByName(this.localPlayerName);
    this.playerId = Math.random().toString(36).substring(2, 8);
    this.remotePlayers = {};
    this.lastSendTime = 0;
    this.sendInterval = 0.1;

    if (!this.localPlayer) {
        console.error("❌ ERRO: Player local '" + this.localPlayerName + "' não encontrado na Hierarchy!");
    }

    this.ws = new WebSocket(this.wsUrl);
    var self = this;

    this.ws.onopen = function() {
        console.log("✅ Conectado ao servidor Go! Seu ID é:", self.playerId);
    };

    this.ws.onmessage = function(event) {
        // 🐞 DEBUG: Mostra a mensagem bruta chegando
        console.log("📩 Mensagem bruta recebida do Go:", event.data); 
        
        try {
            var data = JSON.parse(event.data);
            self.handleMessage(data);
        } catch (e) {
            console.error("Erro ao parsear JSON:", e);
        }
    };

    this.ws.onclose = function() {
        console.warn("❌ Desconectado do servidor.");
        // Remover todos os remote players
        for (var id in self.remotePlayers) {
            self.remotePlayers[id].destroy();
        }
        self.remotePlayers = {};
    };
};

NetworkManager.prototype.handleMessage = function(data) {
    console.log("📩 [NetworkManager] Recebido TIPO:", data.type, "| DADOS:", data);

    // Repassa para o CombatManager
    if (this.entity.script && this.entity.script.combatManager) {
        this.entity.script.combatManager.handleNetworkMessage(data);
    }

    if (data.type === 'move') {
        var id = data.id; 
        
        if (id === this.playerId) {
            return;
        }

        if (!this.remotePlayers[id]) {
            console.log("👥 [MOVE] Spawnando player remoto (via move):", id);
            this.spawnRemotePlayer(id, data.x, data.y, data.z);
        } else {
            var entity = this.remotePlayers[id];
            var targetPos = new pc.Vec3(data.x, data.y, data.z);
            var currentPos = entity.getPosition();
            
            entity.setPosition(
                pc.math.lerp(currentPos.x, targetPos.x, 0.3),
                pc.math.lerp(currentPos.y, targetPos.y, 0.3),
                pc.math.lerp(currentPos.z, targetPos.z, 0.3)
            );
        }
    }
    else if (data.type === 'player_spawn') {
        var id = data.id;
        if (id === this.playerId) {
            console.log("🙈 [SPAWN] Ignorando meu próprio spawn");
            return;
        }
        if (!this.remotePlayers[id]) {
            console.log("👥 [SPAWN] Spawnando player remoto (via player_spawn):", id, "em", data.x, data.y, data.z);
            this.spawnRemotePlayer(id, data.x, data.y, data.z);
        } else {
            console.log("⚠️ [SPAWN] Player remoto já existe:", id);
        }
    }
    else if (data.type === 'player_disconnected') {
        var id = data.id;
        console.log("❌ Player desconectou:", id);
        if (this.remotePlayers[id]) {
            this.remotePlayers[id].destroy();
            delete this.remotePlayers[id];
        }
    }
    // NOVO: Mudança de room
    else if (data.type === 'room_changed') {
        console.log("🚪 Mudou para room:", data.room_id);
        // Limpar remote players e mobs da room antiga
        for (var id in this.remotePlayers) {
            this.remotePlayers[id].destroy();
        }
        this.remotePlayers = {};
        
        // Limpar mobs
        if (this.entity.script.combatManager) {
            for (var mobId in this.entity.script.combatManager.mobs) {
                this.entity.script.combatManager.mobs[mobId].destroy();
            }
            this.entity.script.combatManager.mobs = {};
        }
        
        // Carregar novo mapa (você precisa implementar isso)
        this.loadRoom(data.room_id);
    }
    
    // NOVO: Season reset
    else if (data.type === 'season_reset') {
        console.log("🔄 SEASON RESET! Nova season:", data.level);
        alert("🎉 Nova Season iniciada! Seu level e inventário foram resetados.");
    }
};

// NOVO: Método para trocar de room
NetworkManager.prototype.changeRoom = function(roomId) {
    if (this.ws && this.ws.readyState === WebSocket.OPEN) {
        var msg = JSON.stringify({
            type: 'change_room',
            room_id: roomId
        });
        this.ws.send(msg);
        console.log("🚪 Solicitando mudança para room:", roomId);
    }
};

// NOVO: Método para carregar mapa da room (placeholder)
NetworkManager.prototype.loadRoom = function(roomId) {
    console.log("🗺️ Carregando mapa da room:", roomId);
    // Aqui você carrega o cenário específico da room
    // Por enquanto, só log
};

NetworkManager.prototype.spawnRemotePlayer = function(id, x, y, z) {
    console.log("👥 Spawnando player remoto:", id, "na posição:", x, y, z);
    
    var entity = new pc.Entity("Remote_" + id);
    entity.addComponent('render', {
        type: 'box',
        castShadows: false
    });
    entity.setLocalScale(1, 2, 1);
    entity.setLocalPosition(x, y, z);

    // 🛡️ CORREÇÃO DO MATERIAL (Jeito limpo, sem warnings)
    var mat = new pc.StandardMaterial();
    mat.diffuse = new pc.Color(1, 0, 0); // Vermelho puro
    mat.update();
    entity.render.material = mat;

    this.app.root.addChild(entity);
    this.remotePlayers[id] = entity;
    console.log("✅ Player remoto adicionado à cena com sucesso!");
};

NetworkManager.prototype.update = function(dt) {
    if (!this.localPlayer || !this.ws || this.ws.readyState !== WebSocket.OPEN) return;

    this.lastSendTime += dt;
    if (this.lastSendTime >= this.sendInterval) {
        this.lastSendTime = 0;
        var pos = this.localPlayer.getPosition();
        
        var msg = JSON.stringify({
            type: 'move',
            id: this.playerId,
            x: pos.x,
            y: pos.y,
            z: pos.z
        });
        this.ws.send(msg);
    }
};