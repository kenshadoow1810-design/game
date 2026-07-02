var CombatManager = pc.createScript('combatManager');

CombatManager.attributes.add('attackRange', { type: 'number', default: 5 });
CombatManager.attributes.add('attackCooldown', { type: 'number', default: 0.5 });

CombatManager.prototype.initialize = function() {
    console.log("✅ [CombatManager] Inicializado!");
    
    this.mobs = {};
    this.localPlayer = this.app.root.findByName('Player');
    this.cooldownTimer = 0;
    
    // Habilita o mouse
    this.app.mouse = new pc.Mouse(this.app.graphicsDevice.canvas);
    this.app.mouse.on(pc.EVENT_MOUSEDOWN, this.onMouseDown, this);
    
    console.log("✅ [CombatManager] Mouse habilitado!");
};

CombatManager.prototype.update = function(dt) {
    if (this.cooldownTimer > 0) {
        this.cooldownTimer -= dt;
    }
};

CombatManager.prototype.onMouseDown = function(event) {
    if (event.button !== pc.MOUSEBUTTON_LEFT) return;
    if (this.cooldownTimer > 0) return;

    // 1. Pega a posição do mouse na TELA (2D)
    var mouseX = event.x;
    var mouseY = event.y;
    
    var cameraEntity = this.app.root.findByName('Camera');
    var camera = cameraEntity.camera;
    var playerPos = this.localPlayer.getPosition();
    
    var targetMob = null;
    var minPixelDistance = 60; // 🎯 THRESHOLD: 60 pixels de raio ao redor do mob conta como "clique"
    var minPlayerDistance = this.attackRange;

    console.log("🖱️ Clique na tela em:", mouseX, mouseY);

    // 2. Itera sobre os mobs
    for (var id in this.mobs) {
        var mobEntity = this.mobs[id];
        if (!mobEntity) continue;

        var mobPos = mobEntity.getPosition();
        
        // 3. Verifica se o PLAYER está perto o suficiente para atacar (anti-cheat básico)
        var distToPlayer = playerPos.distance(mobPos);
        if (distToPlayer > this.attackRange) {
            continue; // Player longe demais, ignora
        }

        // 4. O PULO DO GATO: Converte a posição 3D do mob para posição 2D na tela
        var mobScreenPos = new pc.Vec3();
        camera.worldToScreen(mobPos, mobScreenPos);
        
        // 5. Calcula a distância em PIXELS entre o mouse e o mob na tela
        var dx = mouseX - mobScreenPos.x;
        var dy = mouseY - mobScreenPos.y;
        var pixelDistance = Math.sqrt(dx * dx + dy * dy);
        
        console.log("🐔 Mob", id, "| Distância do Player:", distToPlayer.toFixed(2), "| Distância do Mouse (pixels):", pixelDistance.toFixed(2));

        // 6. Se o mouse clicou "em cima" do mob na tela (menor que 60 pixels)
        if (pixelDistance < minPixelDistance) {
            minPixelDistance = pixelDistance;
            targetMob = { id: id, entity: mobEntity };
        }
    }

    // 7. Se achou um mob, ataca!
    if (targetMob) {
        this.cooldownTimer = this.attackCooldown;
        console.log("⚔️ Atacando mob:", targetMob.id);
        
        var msg = JSON.stringify({
            type: 'attack',
            mob_id: targetMob.id
        });
        
        var netManager = this.entity.script.networkManager;
        if (netManager && netManager.ws && netManager.ws.readyState === WebSocket.OPEN) {
            netManager.ws.send(msg);
            console.log("✅ Mensagem de ataque enviada para o Go!");
        }
    } else {
        console.log("❌ Clicou no chão, nenhum mob no alcance do mouse.");
    }
};

// --- PROCESSAMENTO DE MENSAGENS DO SERVIDOR ---

CombatManager.prototype.handleNetworkMessage = function(data) {
    console.log("📩 [CombatManager] Mensagem recebida:", data.type, data);
    
    if (data.type === 'mob_spawn') {
        this.spawnMob(data.mob_id, data.x, data.z);
    } 
    else if (data.type === 'mob_hit') {
        this.onMobHit(data.mob_id, data.hp, data.tagged_by);
    } 
    else if (data.type === 'mob_dead') {
        this.onMobDead(data.mob_id);
    } 
    else if (data.type === 'loot') {
        console.log("💰 LOOT RECEBIDO:", data.amount, "x", data.item);
    }
};

CombatManager.prototype.spawnMob = function(id, x, z) {
    if (this.mobs[id]) {
        console.log("⚠️ Mob", id, "já existe");
        return;
    }

    x = (x !== undefined && x !== null) ? x : 0;
    z = (z !== undefined && z !== null) ? z : -3; 

    console.log("📍 [CombatManager] Spawnando mob em X:", x, "Z:", z);

    var entity = new pc.Entity("Mob_" + id);
    entity.addComponent('render', {
        type: 'box',
        castShadows: false
    });
    
    entity.setLocalScale(1.5, 1.5, 1.5);
    entity.setLocalPosition(x, 1, z); 

    this.app.root.addChild(entity);
    
    var worldPos = entity.getPosition();
    console.log("✅ [CombatManager] Mob criado! Posição mundial:", worldPos.x, worldPos.y, worldPos.z);

    this.mobs[id] = entity;
};

CombatManager.prototype.onMobHit = function(id, hp, taggedBy) {
    var mob = this.mobs[id];
    if (!mob) {
        console.error("❌ Mob", id, "não encontrado para hit");
        return;
    }

    console.log("💥 Mob", id, "levou dano! HP:", hp, "Tag:", taggedBy);
    
    // Pisca vermelho
    var mat = mob.render.material;
    if (!mat) {
        mat = new pc.StandardMaterial();
        mat.diffuse = new pc.Color(0, 1, 0);
        mat.update();
        mob.render.material = mat;
    }
    
    var originalColor = mat.diffuse.clone();
    mat.diffuse = new pc.Color(1, 0, 0);
    mat.update();

    var self = this;
    setTimeout(function() {
        if (mob.render && mob.render.material) {
            mob.render.material.diffuse = originalColor;
            mob.render.material.update();
        }
    }, 150);
};

CombatManager.prototype.onMobDead = function(id) {
    var mob = this.mobs[id];
    if (!mob) {
        console.error("❌ Mob", id, "não encontrado para morte");
        return;
    }

    console.log("💀 Mob morreu:", id);
    
    mob.destroy();
    delete this.mobs[id];
};