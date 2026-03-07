// === Настройки ===
const DB_NAME = "monitoring";
const COLLECTION_NAME = "services";
const USERNAME = "monitoring_user";
const PASSWORD = "monitoring_pass"; // ⚠️ В продакшене — через env/secrets!

// Подключаемся к БД
db = db.getSiblingDB(DB_NAME);

// === 1. Создаём коллекцию с валидацией (пустая) ===
db.createCollection(COLLECTION_NAME, {
  validator: {
    $jsonSchema: {
      bsonType: "object",
      required: ["id", "name", "tenant"],
      properties: {
        id: {
          bsonType: ["int", "string"],
          description: "ID сервиса — обязательный (число или строка)"
        },
        name: {
          bsonType: "string",
          description: "Название сервиса — обязательное"
        },
        tenant: {
          bsonType: "string",
          description: "Тенант — обязательный"
        }
      }
    }
  }
});

print(`✅ Коллекция "${COLLECTION_NAME}" создана (пустая) с валидацией схемы`);

// === 2. Создаём пользователя с правами readWrite ===
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

print(`✅ Пользователь "${USERNAME}" создан с правами readWrite`);

// === 3. Уникальный индекс по id ===
db.services.createIndex({ "id": 1 }, { unique: true });
print("✅ Уникальный индекс по полю 'id' создан");

// === Готово ===
print("========================================");
print("🎉 MongoDB успешно инициализирована!");
print(`БД: ${DB_NAME}`);
print(`Коллекция: ${COLLECTION_NAME} (пустая)`);
print(`Подключайтесь как: mongodb://${USERNAME}:${PASSWORD}@localhost:27017/${DB_NAME}`);
print("Ожидается, что Go-приложение загрузит данные из input.json при первом запуске.");
print("========================================");