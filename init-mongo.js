// === Настройки ===
const DB_NAME = "monitoring";
const COLLECTION_NAME = "services";
const USERNAME = "monitoring_user";
const PASSWORD = "monitoring_pass";

// Подключаемся к БД
db = db.getSiblingDB(DB_NAME);


// === 1. Создаём коллекцию с обновлённой схемой (включая double) ===
db.createCollection(COLLECTION_NAME, {
  validator: {
    $jsonSchema: {
      bsonType: "object",
      required: ["id", "name", "tenant"],
      properties: {
        id: {
          bsonType: ["int", "string", "double"], // ← добавили "double"
          description: "ID сервиса — обязательный (целое число, строка или double)"
        },
        name: {
          bsonType: "string",
          description: "Название сервиса — обязательное"
        },
        tenant: {
          bsonType: "string",
          description: "Тенант — обязательный"
        },
        clusters: {
          bsonType: "array",
          description: "Список кластеров (опционально)",
          items: {
            bsonType: "string"
          }
        }
      }
    }
  }
});

print(`✅ Коллекция "${COLLECTION_NAME}" создана с обновлённой схемой (поддержка double)`);

// === 2. Создаём пользователя (если ещё не существует) ===
if (!db.getUser(USERNAME)) {
    db.createUser({
        user: USERNAME,
        pwd: PASSWORD,
        roles: [
            {
                role: "readWrite",
                db: DB_NAME
            }
        ]
    });
    print(`✅ Пользователь "${USERNAME}" создан`);
} else {
    print(`ℹ️  Пользователь "${USERNAME}" уже существует`);
}

// === 3. Создаём уникальный индекс по id ===
db[COLLECTION_NAME].createIndex({ "id": 1 }, { unique: true });
print("✅ Уникальный индекс по полю 'id' создан");

// === Готово ===
print("========================================");
print("🎉 Настройка MongoDB завершена!");
print(`Подключайтесь как: mongodb://${USERNAME}:${PASSWORD}@localhost:27017/${DB_NAME}`);
print("Ожидается, что приложение загрузит данные из JSON и обогатит их.");
print("========================================");