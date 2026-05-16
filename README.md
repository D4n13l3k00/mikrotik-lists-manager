# mikrotik-lists-manager

CLI утилита для управления firewall address-list на MikroTik из локального файла.

Подключается к роутеру через REST API (RouterOS 7+), сравнивает текущее состояние списка с файлом и приводит его к нужному виду — добавляет, удаляет, обновляет и включает/отключает записи. Динамические записи (`dynamic=true`) не трогает.

## Требования

- RouterOS 7.x (REST API)
- Go 1.21+ для сборки

## Установка

```bash
# скачать готовый бинарь со страницы релизов
https://github.com/D4n13l3k00/mikrotik-lists-manager/releases

# или собрать из исходников
git clone https://github.com/D4n13l3k00/mikrotik-lists-manager
cd mikrotik-lists-manager
go build -o mikrotik-lists-manager ./cmd/main/
```

## Быстрый старт

```bash
# посмотреть что изменится, не применяя
./mikrotik-lists-manager sync list.lst -H 192.168.1.1 -u admin -l vpn-routes -n

# применить
./mikrotik-lists-manager sync list.lst -H 192.168.1.1 -u admin -l vpn-routes

# посмотреть все списки на роутере
./mikrotik-lists-manager list -H 192.168.1.1 -u admin
```

Пароль будет запрошен интерактивно если не передан флагом `-p`.

---

## Формат файла

Поддерживаются два формата.

### Native (`.list`)

Основной формат. Каждая строка — IP-адрес, CIDR или домен.

| Синтаксис | Значение |
|-----------|----------|
| `# текст` | Комментарий только локально, в MikroTik не попадает |
| `## текст` | Становится полем `comment` записи в MikroTik |
| `!адрес` | Запись будет синхронизирована как `disabled=true` |

`##` может стоять в начале строки (применяется к следующей записи) или inline после адреса.

```
# это заметка только для себя — в MikroTik не уйдёт

## GOOGLE DNS
8.8.8.8
8.8.4.4

1.1.1.1   ## CLOUDFLARE DNS
!2.2.2.2  ## ВРЕМЕННО ОТКЛЮЧЕНО

# --- подсети Telegram ---
91.108.4.0/22    ## TELEGRAM
149.154.160.0/20 ## TELEGRAM
```

### MikroTik export (`.rsc`)

Формат вывода команды `/export` на роутере. Удобно для импорта существующего списка.

```
/ip firewall address-list
add list=vpn-routes address=8.8.8.8 comment="GOOGLE DNS"
add list=vpn-routes address=1.1.1.1 comment="CLOUDFLARE DNS"
add list=vpn-routes address=91.108.4.0/22 comment="TELEGRAM"
```

Формат определяется автоматически (`-f auto`). Можно указать явно: `-f native` или `-f mikrotik`.

---

## Несколько списков

Все команды принимают несколько списков через запятую или повторением флага:

```bash
./mikrotik-lists-manager sync list.lst -l vpn,blocked
./mikrotik-lists-manager sync list.lst -l vpn -l blocked
./mikrotik-lists-manager disable -a -l vpn,blocked,whitelist
```

---

## Команды

### `sync` — полная синхронизация

Читает файл, получает текущий список с роутера, вычисляет diff и приводит список на роутере к точному состоянию файла: добавляет отсутствующие, удаляет лишние, обновляет комментарии и состояние `disabled`.

Если запись в файле без `!`, но на роутере она `disabled=true` — включит обратно.

При 10+ изменениях показывает прогресс-бар. Флаг `-v` включает построчный вывод вместе с баром.

```
./mikrotik-lists-manager sync [file] [флаги]
```

| Флаг | Короткий | Описание |
|------|----------|----------|
| `--host` | `-H` | Адрес роутера: `192.168.1.1`, `http://10.0.0.1`, `https://host:8443` |
| `--user` | `-u` | Имя пользователя API |
| `--pass` | `-p` | Пароль (если не задан — запросит интерактивно) |
| `--list` | `-l` | Имя address-list, можно несколько: `-l a,b` или `-l a -l b` |
| `--format` | `-f` | Формат файла: `auto`, `native`, `mikrotik` (по умолчанию `auto`) |
| `--dry-run` | `-n` | Показать изменения без применения |
| `--verbose` | `-v` | Выводить каждую запись даже при прогресс-баре |
| `--insecure` | `-k` | Не проверять TLS сертификат |

```bash
# обычная синхронизация
./mikrotik-lists-manager sync vpn.list -H 192.168.1.1 -u admin -l vpn-routes

# dry-run — только посмотреть diff
./mikrotik-lists-manager sync vpn.list -H 192.168.1.1 -u admin -l vpn-routes -n

# синхронизировать в несколько списков
./mikrotik-lists-manager sync vpn.list -H 192.168.1.1 -u admin -l vpn,blocked

# роутер на HTTP
./mikrotik-lists-manager sync vpn.list -H http://192.168.1.1 -u admin -l vpn-routes

# нестандартный порт, самоподписанный сертификат
./mikrotik-lists-manager sync vpn.list -H https://192.168.1.1:8443 -u admin -l vpn-routes -k

# из stdin
cat vpn.list | ./mikrotik-lists-manager sync - -H 192.168.1.1 -u admin -l vpn-routes

# с подробным выводом при большом списке
./mikrotik-lists-manager sync vpn.list -H 192.168.1.1 -u admin -l vpn-routes -v
```

---

### `list` — просмотр списков на роутере

Показывает все address-list на роутере с количеством записей и сколько из них отключено.

```
./mikrotik-lists-manager list [флаги]
```

| Флаг | Короткий | Описание |
|------|----------|----------|
| `--host` | `-H` | Адрес роутера |
| `--user` | `-u` | Имя пользователя API |
| `--pass` | `-p` | Пароль |
| `--insecure` | `-k` | Не проверять TLS сертификат |

```bash
./mikrotik-lists-manager list -H 192.168.1.1 -u admin
```

---

### `append` — добавление записей

Добавляет в список на роутере только те записи из файла, которых там ещё нет. Существующие записи не трогает и не обновляет.

```
./mikrotik-lists-manager append [file] [флаги]
```

| Флаг | Короткий | Описание |
|------|----------|----------|
| `--host` | `-H` | Адрес роутера |
| `--user` | `-u` | Имя пользователя API |
| `--pass` | `-p` | Пароль |
| `--list` | `-l` | Имя address-list, можно несколько |
| `--format` | `-f` | Формат файла: `auto`, `native`, `mikrotik` |
| `--dry-run` | `-n` | Показать изменения без применения |
| `--insecure` | `-k` | Не проверять TLS сертификат |

```bash
# добавить новые записи, дубли пропустить
./mikrotik-lists-manager append extra.list -H 192.168.1.1 -u admin -l vpn-routes

# добавить в несколько списков сразу
./mikrotik-lists-manager append extra.list -H 192.168.1.1 -u admin -l vpn,blocked
```

---

### `remove` — удаление записей по файлу

Удаляет с роутера только те записи, которые перечислены в файле. Остальные записи в списке не трогает. Если адрес из файла не найден на роутере — выводит предупреждение.

```
./mikrotik-lists-manager remove [file] [флаги]
```

| Флаг | Короткий | Описание |
|------|----------|----------|
| `--host` | `-H` | Адрес роутера |
| `--user` | `-u` | Имя пользователя API |
| `--pass` | `-p` | Пароль |
| `--list` | `-l` | Имя address-list, можно несколько |
| `--format` | `-f` | Формат файла: `auto`, `native`, `mikrotik` |
| `--dry-run` | `-n` | Показать изменения без применения |
| `--insecure` | `-k` | Не проверять TLS сертификат |

```bash
# удалить записи из файла, остальные оставить
./mikrotik-lists-manager remove telegram.list -H 192.168.1.1 -u admin -l vpn-routes

# посмотреть что удалится
./mikrotik-lists-manager remove telegram.list -H 192.168.1.1 -u admin -l vpn-routes -n
```

---

### `export` — экспорт

Получает текущий список с роутера и выводит его в stdout или файл. При нескольких списках и `-o` все списки записываются в один файл подряд.

```
./mikrotik-lists-manager export [флаги]
```

| Флаг | Короткий | Описание |
|------|----------|----------|
| `--host` | `-H` | Адрес роутера |
| `--user` | `-u` | Имя пользователя API |
| `--pass` | `-p` | Пароль |
| `--list` | `-l` | Имя address-list, можно несколько |
| `--format` | `-f` | Формат вывода: `native` (по умолчанию), `mikrotik` |
| `--output` | `-o` | Записать в файл вместо stdout |
| `--insecure` | `-k` | Не проверять TLS сертификат |

```bash
# вывести в терминал
./mikrotik-lists-manager export -H 192.168.1.1 -u admin -l vpn-routes

# сохранить несколько списков в один файл
./mikrotik-lists-manager export -H 192.168.1.1 -u admin -l vpn,blocked -o backup.list

# сохранить в формате MikroTik export
./mikrotik-lists-manager export -H 192.168.1.1 -u admin -l vpn-routes -f mikrotik -o backup.rsc
```

---

### `enable` / `disable` — включение и отключение

Включает или отключает конкретные записи или весь список на роутере.
**Не изменяет локальный файл** — только состояние на роутере.

```
./mikrotik-lists-manager enable [адрес...] [флаги]
./mikrotik-lists-manager disable [адрес...] [флаги]
```

| Флаг | Короткий | Описание |
|------|----------|----------|
| `--host` | `-H` | Адрес роутера |
| `--user` | `-u` | Имя пользователя API |
| `--pass` | `-p` | Пароль |
| `--list` | `-l` | Имя address-list, можно несколько |
| `--all` | `-a` | Применить ко всем записям списка |
| `--insecure` | `-k` | Не проверять TLS сертификат |

```bash
# отключить конкретные записи
./mikrotik-lists-manager disable 8.8.8.8 1.1.1.1 -H 192.168.1.1 -u admin -l vpn-routes

# включить конкретные записи
./mikrotik-lists-manager enable 8.8.8.8 -H 192.168.1.1 -u admin -l vpn-routes

# отключить несколько списков целиком
./mikrotik-lists-manager disable -a -H 192.168.1.1 -u admin -l vpn,blocked

# включить весь список
./mikrotik-lists-manager enable -a -H 192.168.1.1 -u admin -l vpn-routes
```

---

### `optimize` — оптимизация файла

Читает native `.list` файл и удаляет:
- дублирующиеся адреса и домены
- IP/CIDR которые полностью покрываются более широкой подсетью в том же файле

По умолчанию выводит результат в stdout. С флагом `-w` перезаписывает файл.

```
./mikrotik-lists-manager optimize [file] [флаги]
```

| Флаг | Короткий | Описание |
|------|----------|----------|
| `--write` | `-w` | Перезаписать файл вместо вывода в stdout |

```bash
# посмотреть что будет удалено
./mikrotik-lists-manager optimize list.lst

# применить оптимизацию
./mikrotik-lists-manager optimize list.lst -w
```

---

### `config` — управление конфигом

#### `config init`

Создаёт шаблон конфига `.mikrotik-lists-manager.yaml` в текущей директории.

```bash
./mikrotik-lists-manager config init

# указать другой путь
./mikrotik-lists-manager config init --config /etc/vpn/config.yaml
```

#### `config show`

Показывает итоговую конфигурацию с учётом файла и переменных окружения. Пароль маскируется.

```bash
./mikrotik-lists-manager config show
```

---

## Конфигурационный файл

Утилита ищет `.mikrotik-lists-manager.yaml` в текущей директории. Путь можно переопределить флагом `--config`.

Приоритет для каждого параметра: **флаг > переменная окружения > конфиг файл**.

```yaml
# .mikrotik-lists-manager.yaml
host: "192.168.1.1"
user: "admin"
pass: ""            # лучше оставить пустым — спросит при запуске
list: "vpn-routes"
insecure: false
default_format: auto
```

После создания конфига команды становятся короче:

```bash
./mikrotik-lists-manager sync list.lst -n
./mikrotik-lists-manager sync list.lst
./mikrotik-lists-manager list
./mikrotik-lists-manager append extra.list
./mikrotik-lists-manager remove telegram.list -n
./mikrotik-lists-manager export -o backup.list
./mikrotik-lists-manager disable -a
```

---

## Переменные окружения

| Переменная | Флаг |
|------------|------|
| `MT_HOST` | `-H` / `--host` |
| `MT_USER` | `-u` / `--user` |
| `MT_PASS` | `-p` / `--pass` |
| `MT_LIST` | `-l` / `--list` |

```bash
export MT_HOST=192.168.1.1
export MT_USER=admin
export MT_LIST=vpn-routes

./mikrotik-lists-manager sync list.lst -n
./mikrotik-lists-manager list
./mikrotik-lists-manager disable -a
```

---

## Сравнение команд

| Команда | Добавляет | Удаляет | Обновляет | Трогает только из файла |
|---------|-----------|---------|-----------|------------------------|
| `sync` | ✓ | ✓ | ✓ | — (полная синхронизация) |
| `append` | ✓ | — | — | ✓ |
| `remove` | — | ✓ | — | ✓ |

---

## Структура проекта

```
mikrotik-lists-manager/
├── cmd/
│   └── main/
│       └── main.go              — точка входа
└── internal/
    ├── cli/
    │   ├── root.go              — cobra root, config команды, кастомный help
    │   ├── sync.go              — команда sync
    │   ├── append_remove.go     — команды append / remove
    │   ├── export.go            — команда export
    │   ├── list.go              — команда list
    │   ├── toggle.go            — команды enable / disable
    │   └── optimize.go          — команда optimize
    ├── config/
    │   └── config.go            — загрузка/сохранение YAML конфига
    ├── mikrotik/
    │   └── client.go            — REST API клиент
    ├── optimizer/
    │   └── optimizer.go         — дедупликация и суммаризация подсетей
    ├── output/
    │   └── output.go            — цветной вывод (lipgloss)
    ├── parser/
    │   ├── entry.go             — тип Entry
    │   ├── parser.go            — парсинг native и mikrotik форматов
    │   └── parser_test.go
    └── syncer/
        └── syncer.go            — diff логика, прогресс-бар, применение изменений
```

---

## Настройка MikroTik

Для работы REST API нужен пользователь с правами на чтение/запись firewall:

```
/user group add name=api-sync policy=read,write,api,rest-api
/user add name=sync group=api-sync password=yourpassword
```

REST API включён по умолчанию в RouterOS 7. Проверить активные сервисы:

```
/ip service print
```

Должен быть активен `www-ssl` (порт 443) или `www` (порт 80).
