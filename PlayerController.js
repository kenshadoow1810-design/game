var PlayerController = pc.createScript('playerController');

// Atributos editáveis no editor
PlayerController.attributes.add('moveSpeed', { type: 'number', default: 8 });
PlayerController.attributes.add('camera', { type: 'entity', title: 'Camera Entity' });

// Inicialização (Roda 1 vez só)
PlayerController.prototype.initialize = function() {
    this.direction = new pc.Vec3();
    this.right = new pc.Vec3();
    this.forward = new pc.Vec3();
    this.cameraOffset = new pc.Vec3(); // Variável para guardar a distância da câmera

    // Busca a câmera automaticamente se o atributo não foi preenchido na UI
    if (!this.camera) {
        this.camera = this.app.root.findByName('Camera'); 
    }
    
    if (this.camera) {
        // A MÁGICA ACONTECE AQUI:
        // Calculamos a distância entre a câmera e o player UMA VEZ SÓ no início.
        var camPos = this.camera.getPosition();
        var playerPos = this.entity.getPosition();
        this.cameraOffset.sub2(camPos, playerPos);
        
        console.log("✅ Câmera vinculada! Offset inicial calculado:", this.cameraOffset.toString());
    } else {
        console.error("❌ Câmera não encontrada! Verifique o nome dela na Hierarchy.");
    }
};

// Loop principal (Roda todo frame)
PlayerController.prototype.update = function(dt) {
    var app = this.app;
    var entity = this.entity;
    
    // 1. Capturar Input (WASD)
    var inputX = 0;
    var inputZ = 0;
    
    if (app.keyboard.isPressed(pc.KEY_W)) inputZ = -1;
    if (app.keyboard.isPressed(pc.KEY_S)) inputZ = 1;
    if (app.keyboard.isPressed(pc.KEY_A)) inputX = -1;
    if (app.keyboard.isPressed(pc.KEY_D)) inputX = 1;
    
    // 2. Calcular direção baseada na câmera (movimento relativo à tela)
    if (this.camera) {
        var cameraRotation = this.camera.getEulerAngles();
        
        this.forward.set(
            -Math.sin(cameraRotation.y * pc.math.DEG_TO_RAD),
            0,
            -Math.cos(cameraRotation.y * pc.math.DEG_TO_RAD)
        );
        
        this.right.set(
            Math.cos(cameraRotation.y * pc.math.DEG_TO_RAD),
            0,
            -Math.sin(cameraRotation.y * pc.math.DEG_TO_RAD)
        );
        
        this.direction.set(0, 0, 0);
        
        if (inputZ !== 0) {
            var moveForward = this.forward.clone().scale(-inputZ);
            this.direction.add(moveForward);
        }
        
        if (inputX !== 0) {
            var moveRight = this.right.clone().scale(inputX);
            this.direction.add(moveRight);
        }
        
        if (this.direction.length() > 0) {
            this.direction.normalize();
        }
    } else {
        this.direction.set(inputX, 0, inputZ);
    }

    // 3. Aplicar Movimento no Player
    var movement = this.direction.clone().scale(this.moveSpeed * dt);
    entity.translate(movement);
    
    // 4. FOLLOW CAMERA CORRIGIDO
    if (this.camera) {
        var playerPos = entity.getPosition();
        
        // A posição alvo da câmera é a posição ATUAL do player + o OFFSET INICIAL
        var targetCamPos = new pc.Vec3().add2(playerPos, this.cameraOffset);
        
        var camPos = this.camera.getPosition();
        
        // Suaviza o movimento da câmera (Lerp) para não ficar travado
        this.camera.setPosition(
            pc.math.lerp(camPos.x, targetCamPos.x, 0.1),
            pc.math.lerp(camPos.y, targetCamPos.y, 0.1),
            pc.math.lerp(camPos.z, targetCamPos.z, 0.1)
        );
    }
};