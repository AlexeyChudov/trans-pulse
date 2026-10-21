# Runbook: восстановление PostgreSQL (primary + replica)

Стенд (`lab4/EXPERIMENT-TEMPLATE`): `primary` (порт 55432) и `replica` (порт 55433), асинхронная потоковая репликация, PostgreSQL 17.
Приложение подключается к обоим серверам сразу (`target_session_attrs=read-write`) и пишет в тот, который принимает запись.
Команды `docker compose exec … psql -U course_admin` выполняются **внутри контейнера** (локальный сокет). Запросы по сети: `psql "postgresql://course_admin:<пароль из .env>@127.0.0.1:<порт>/course_app?sslmode=require"`.
Все команды выполняются из папки `lab4/EXPERIMENT-TEMPLATE`.

---

## 1. Недоступна replica

**Признаки:** приложение работает без ошибок (оно пишет на primary), но аналитические запросы на replica не проходят; на primary `SELECT count(*) FROM pg_stat_replication;` возвращает 0.

**Диагностика**
1. `docker compose ps -a replica` — контейнер остановлен или перезапускается?
2. `docker compose logs --tail 50 replica` — причина падения.
3. На primary: `SELECT slot_name, active FROM pg_replication_slots;` — слот неактивен, пока реплики нет.

**Восстановление**
1. `docker compose start replica` — реплика сама догонит primary по потоку WAL.
2. Если данные реплики повреждены: `docker compose rm -sf replica && docker volume rm sprint04_replica_data && docker compose up -d replica` — снимется новая базовая копия.

**Проверка:** `SELECT state, replay_lag FROM pg_stat_replication;` → `streaming`, лаг около нуля; число строк в ключевых таблицах совпадает на обоих узлах.

---

## 2. Недоступен primary

**Признаки:** приложение получает ошибки подключения (`connection refused`, `server closed the connection`, `session is read-only`); `pg_isready -h 127.0.0.1 -p 55432` → `no response`.

**Диагностика**
1. `docker compose ps -a primary`, `docker compose logs --tail 100 primary` — авария, нехватка диска, OOM?
2. Если primary можно просто перезапустить (`docker compose start primary`) и он поднялся — переключаться не нужно, это самый простой путь.
3. Состояние replica: `docker compose exec -T replica psql -U course_admin -d course_app -Atc "SELECT pg_is_in_recovery()"` → `t`; `pg_stat_wal_receiver` — связи с primary нет.
4. Убедиться, что **старый primary действительно мёртв** и не вернётся (иначе два primary, «split-brain»).

**Восстановление (переключение)**
1. `docker compose exec -T replica psql -U course_admin -d course_app -Atc "SELECT pg_promote(wait => true)"`.
2. Приложение переподключится само (оно перечисляет оба сервера). Если оно использует один адрес — заменить адрес на новый primary.
3. Старый primary «как есть» не возвращать: его данные расходятся с новым. Пересобрать как реплику нового primary (команды в `scripts/failure_primary.sh`, раздел «Возвращаем старый primary»): удалить его том, снять `pg_basebackup` с нового primary, запустить `docker compose up -d primary`.
4. Роли поменялись: primary — порт 55433, replica — порт 55432.

**Допустимая потеря данных.** Репликация асинхронная: записи, не успевшие дойти до replica в момент аварии, теряются (в опыте — 0, лаг порядка миллисекунд). Если потеря недопустима, нужна синхронная репликация (`synchronous_standby_names`) ценой задержки записи.

**Проверка целостности**
1. `SELECT pg_is_in_recovery();` на новом primary → `f`.
2. Число строк в таблицах равно ожидаемому (последнее известное значение / резервная копия).
3. Нет «висячих» ссылок: `SELECT count(*) FROM trips t WHERE NOT EXISTS (SELECT 1 FROM routes r WHERE r.id=t.route_id);` → 0 (то же для `vehicle_id`).
4. Нет автобусов с двумя активными рейсами: `SELECT vehicle_id FROM trips WHERE status='IN_PROGRESS' GROUP BY 1 HAVING count(*)>1;` → пусто.
5. `sql/05-integrity-check.sql` (проба репликации) проходит; тестовая запись от имени приложения появляется на пересобранной реплике.

---

## 3. Потеряны оба узла или повреждены данные

**Признаки:** оба контейнера не стартуют, либо данные ошибочно удалены и это уже реплицировалось.

**Восстановление из резервной копии**
1. Копии: `backups/course_app.dump` (данные) и `backups/globals.sql` (роли без паролей). Создаются `scripts/backup.sh`; восстановление — `scripts/restore.sh`.
2. В чистом экземпляре PostgreSQL 17: сначала роли (`psql -f globals_clean.sql`, строки про суперпользователя убрать; пароли задать заново из `.env`), затем `pg_restore --dbname=<строка подключения> course_app.dump`.
3. Поднять replica заново из восстановленного primary (раздел 1, п. 2).

**Проверка:** число строк и контрольная сумма (`lab3/sql/checksum.sql`) совпадают с исходной базой; внешних ключей столько же; RLS-политики на месте (`SELECT count(*) FROM pg_policies;` → 3).
**Допустимая потеря:** всё, что записано после момента снятия копии.

---

## 4. Памятка

| Вопрос | Команда |
|---|---|
| Кто primary | `SELECT pg_is_in_recovery();` на каждом узле (`f` — primary) |
| Отставание реплики | на primary: `SELECT replay_lag, pg_wal_lsn_diff(pg_current_wal_lsn(), replay_lsn) FROM pg_stat_replication;` |
| Кто кого блокирует | `SELECT pid, pg_blocking_pids(pid), wait_event, query FROM pg_stat_activity WHERE cardinality(pg_blocking_pids(pid))>0;` |
| Что менялось (критические операции) | `SELECT * FROM app.audit_log ORDER BY id DESC LIMIT 20;` |
