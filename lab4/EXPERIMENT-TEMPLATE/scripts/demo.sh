#!/bin/bash
# Экскурсия по лабораторной 4: одна команда, шаги по порядку отчёта, на каждом шаге подпись «что смотреть».
#   bash scripts/demo.sh            — с паузами (Enter — следующий шаг)
#   bash scripts/demo.sh --auto     — без пауз
#   bash scripts/demo.sh --no-reset — не пересоздавать стенд (по умолчанию он создаётся заново, ~40 с)
cd "$(dirname "$0")/.."
AUTO=0; RESET=1
for a in "$@"; do [ "$a" = "--auto" ] && AUTO=1; [ "$a" = "--no-reset" ] && RESET=0; done
[ -f .env ] || ./scripts/init_env.sh >/dev/null
source scripts/lib.sh
TMP=$(mktemp -d); trap 'rm -rf "$TMP"' EXIT

B=$'\e[1m'; G=$'\e[32m'; Y=$'\e[33m'; N=$'\e[0m'
step() { echo; echo "${B}════════ Шаг $1 из 8. $2${N}"; echo "${Y}Что показываем:${N} $3"; echo "${Y}На что смотреть:${N} $4"; echo; }
pause() { [ $AUTO = 1 ] || read -r -p $'\n'"${G}Нажмите Enter для следующего шага…${N}" _ ; }
# section <номер> <файл>: напечатать только раздел «== N. …» из вывода скрипта
section() { awk -v n="$1" '$0 ~ "^== "n"\\." {p=1; print; next} /^== / {p=0} p' "$2"; }
run() { bash "scripts/$1.sh" > "$TMP/$1.txt" 2>&1; }

echo "${B}Лабораторная 4: роли, backup/restore, репликация, отказы PostgreSQL${N}"
if [ $RESET = 1 ]; then
  echo "Готовим чистый стенд (primary + replica, схема и данные проекта)… около 40 секунд"
  ./scripts/reset.sh > "$TMP/reset.txt" 2>&1 || { echo "Не удалось поднять стенд. Запущен ли Docker? Подробности: $TMP/reset.txt"; cat "$TMP/reset.txt" | tail -15; trap - EXIT; exit 1; }
  echo "Стенд готов: primary = порт $PRIMARY, replica = порт $REPLICA."
fi

step 1 "Роли и права (раздел 1 отчёта)" \
  "у приложения нет прав суперпользователя, каждая роль может только своё; небезопасное подключение отклоняется" \
  "ЗАПРЕЩЕНО у course_app для привилегированных и структурных операций; аналитик не видит password_hash; sslmode=disable отклонён"
run demo_roles
section 2 "$TMP/demo_roles.txt"
echo; echo "Ещё из проверок:"
grep -E "DROP TABLE routes|TRUNCATE trips|password_hash|INSERT INTO routes" "$TMP/demo_roles.txt"
echo; section 6 "$TMP/demo_roles.txt"
pause

step 2 "Секреты (раздел 2 отчёта)" \
  "пароли не лежат в репозитории: в git только шаблон, настоящий .env скрыт" \
  ".env.example содержит change_me; .env помечен как игнорируемый (правило .gitignore); права файла 600"
echo "— шаблон паролей (в git):"; grep PASSWORD .env.example
echo; echo "— настоящий файл .env скрыт от git, правило:"; git check-ignore -v .env
echo; echo "— права файла (только владелец):"; ls -l .env | awk '{print $1, $NF}'
pause

step 3 "Резервная копия и восстановление в чистый экземпляр (раздел 3 отчёта)" \
  "делаем копию БД, поднимаем новый пустой сервер, восстанавливаем и сверяем с оригиналом" \
  "в новой БД до восстановления 0 таблиц; потом число строк совпадает, контрольные суммы совпадают («СОВПАДАЕТ»)"
run backup; run restore
cat "$TMP/backup.txt"; echo; cat "$TMP/restore.txt"
pause

step 4 "Репликация primary → replica (раздел 4 отчёта)" \
  "изменения идут с главного сервера на копию, а в обратную сторону писать нельзя" \
  "primary = f, replica = t; запись пробы на primary появилась на replica; запись на replica отклонена"
run demo_replication
cat "$TMP/demo_replication.txt"
pause

step 5 "Дополнительно: RLS и аудит (раздел 7 отчёта)" \
  "диспетчер может закрыть инцидент только от своего имени; все критические действия попадают в журнал" \
  "чужое имя и закрытие без имени: ЗАПРЕЩЕНО; своё имя: РАЗРЕШЕНО; в audit_log видно кто, что и когда изменил"
run demo_rls_audit
cat "$TMP/demo_rls_audit.txt"
pause

step 6 "Дополнительно: аналитика на replica (раздел 7 отчёта)" \
  "аналитик работает только с replica, главный сервер не нагружается" \
  "это replica (t); отчёты выполняются; запись запрещена; сессий аналитика на primary 0"
run analytics_on_replica
grep -E "replica|Time:|ЗАПРЕЩЕНО|Сессий" "$TMP/analytics_on_replica.txt"
pause

step 7 "Отказ 1: остановка replica (раздел 5 отчёта)" \
  "останавливаем копию — приложение продолжает работать, потом копия догоняет" \
  "подключённых реплик 0, но в журнале приложения FAIL=0; после запуска replica «streaming» и строки на обоих узлах равны"
run failure_replica
cat "$TMP/failure_replica.txt"
pause

step 8 "Отказ 2: падение primary и переключение (раздел 5 отчёта)" \
  "«убиваем» главный сервер, переводим replica в главные, проверяем данные, возвращаем старый сервер как копию" \
  "приложение ловит ошибки несколько секунд и само переключается; «потеряно записей: 0»; висячих ссылок 0; старый primary вернулся как replica"
run failure_primary
cat "$TMP/failure_primary.txt" | grep -v "Pager usage"

echo; echo "${B}Готово.${N} Разделы 6 (runbook) и 9 (артефакты) — это документы: docs/lab4/runbook.md и docs/lab4/report.md."
echo "Замер replication lag (занимает ~1 минуту, грузит 3 млн рейсов): bash scripts/lag_test.sh"
echo "После шага 8 серверы поменялись ролями. Чтобы повторить экскурсию, просто запустите demo.sh снова (стенд создастся заново)."
