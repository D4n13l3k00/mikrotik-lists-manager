# 🛠 mikrotik-lists-manager

CLI утилита для управления firewall address-list на MikroTik из локального файла.

Подключается к роутеру через REST API (RouterOS 7+), сравнивает текущее состояние списка с файлом и приводит его к нужному виду — добавляет, удаляет, обновляет и включает/отключает записи. Динамические записи (`dynamic=true`) не трогает.

## Содержание

- [Установка](#установка)
- [Быстрый старт](#быстрый-старт)
- [Формат файла](#формат-файла)
- [Несколько списков](#несколько-списков)
- [Команды](#команды)
  - [sync](#sync--полная-синхронизация)
  - [list](#list--просмотр-списков-на-роутере)
  - [append](#append--добавление-записей)
  - [remove](#remove--удаление-записей-по-файлу)
  - [export](#export--экспорт)
  - [enable / disable](#enable--disable--включение-и-отключение)
  - [optimize](#optimize--оптимизация-файла)
  - [fetch](#fetch--автозагрузка-cidr-диапазонов)
  - [find](#find--поиск-адреса-на-роутере)
  - [backup](#backup--резервное-копирование)
  - [rename](#rename--переименование-списка)
  - [info](#info--информация-о-роутере)
  - [completion](#completion--автодополнение-оболочки)
  - [config](#config--управление-конфигом)
- [Конфигурационный файл](#конфигурационный-файл)
- [Переменные окружения](#переменные-окружения)
- [Сравнение команд](#сравнение-команд)
- [Настройка MikroTik](#настройка-mikrotik)

---

## 📦 Установка

Скачать готовый бинарь для своей платформы со [страницы релизов](https://github.com/D4n13l3k00/mikrotik-lists-manager/releases).

Или собрать из исходников:

```bash
git clone https://github.com/D4n13l3k00/mikrotik-lists-manager
cd mikrotik-lists-manager
go build -o mikrotik-lists-manager ./cmd/main/
```

**Требования:** RouterOS 7.x (REST API), Go 1.21+ для сборки.

---

## 🚀 Быстрый старт

```bash
# посмотреть что изменится, не применяя
./mikrotik-lists-manager sync list.lst -H 192.168.1.1 -u admin -l vpn-routes -n

# применить
./mikrotik-lists-manager sync list.lst -H 192.168.1.1 -u admin -l vpn-routes

# посмотреть все списки на роутере
./mikrotik-lists-manager list -H 192.168.1.1 -u admin

# скачать актуальные CIDR от провайдеров
./mikrotik-lists-manager fetch -o ranges.lst
```

Пароль будет запрошен интерактивно если не передан флагом `-p`.

---

## 📄 Формат файла

Поддерживаются два формата.

### Native (`.list`)

Основной формат. Каждая строка — IP-адрес, CIDR или домен.

| Синтаксис | Значение |
|-----------|----------|
| `# текст` | Комментарий только локально, в MikroTik не попадает |
| `## текст` | Становится полем `comment` записи в MikroTik |
| `!адрес` | Запись будет синхронизирована как `disabled=true` |

`##` может стоять в начале строки (применяется к следующей записи) или inline после адреса.

```text
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

```routeros
/ip firewall address-list
add list=vpn-routes address=8.8.8.8 comment="GOOGLE DNS"
add list=vpn-routes address=1.1.1.1 comment="CLOUDFLARE DNS"
add list=vpn-routes address=91.108.4.0/22 comment="TELEGRAM"
```

Формат определяется автоматически (`-f auto`). Можно указать явно: `-f native` или `-f mikrotik`.

---

## 🗂 Несколько списков

Все команды принимают несколько списков через запятую или повторением флага:

```bash
./mikrotik-lists-manager sync list.lst -l vpn,blocked
./mikrotik-lists-manager sync list.lst -l vpn -l blocked
./mikrotik-lists-manager disable -a -l vpn,blocked,whitelist
```

---

## 📋 Команды

### `sync` — полная синхронизация

Читает файл, получает текущий список с роутера, вычисляет diff и приводит список на роутере к точному состоянию файла: добавляет отсутствующие, удаляет лишние, обновляет комментарии и состояние `disabled`.

Если запись в файле без `!`, но на роутере она `disabled=true` — включит обратно.
При 10+ изменениях показывает прогресс-бар. Флаг `-v` включает построчный вывод вместе с баром.

```shell
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
| `--concurrency` | `-c` | Число параллельных запросов к API (по умолчанию 5, 0 = последовательно) |
| `--watch` | `-w` | Следить за файлом и пересинхронизировать при изменении |
| `--watch-interval` | | Интервал проверки файла в секундах (по умолчанию 3, с `--watch`) |
| `--insecure` | `-k` | Не проверять TLS сертификат |

```bash
# обычная синхронизация
./mikrotik-lists-manager sync vpn.list -H 192.168.1.1 -u admin -l vpn-routes

# dry-run — только посмотреть diff
./mikrotik-lists-manager sync vpn.list -H 192.168.1.1 -u admin -l vpn-routes -n

# синхронизировать в несколько списков
./mikrotik-lists-manager sync vpn.list -H 192.168.1.1 -u admin -l vpn,blocked

# из stdin
cat vpn.list | ./mikrotik-lists-manager sync - -H 192.168.1.1 -u admin -l vpn-routes

# следить за файлом и синхронизировать при каждом изменении
./mikrotik-lists-manager sync vpn.list -H 192.168.1.1 -u admin -l vpn-routes -w
```

---

### `list` — просмотр списков на роутере

Показывает все address-list на роутере с количеством записей и сколько из них отключено.

```shell
./mikrotik-lists-manager list [флаги]
```

| Флаг | Короткий | Описание |
|------|----------|----------|
| `--host` | `-H` | Адрес роутера |
| `--user` | `-u` | Имя пользователя API |
| `--pass` | `-p` | Пароль |
| `--entries` | `-e` | Показать все записи конкретного списка |
| `--sort` | | Сортировка: `name` (по умолчанию) или `size` (по количеству записей) |
| `--filter` | `-F` | Фильтр по имени списка (подстрока, без учёта регистра) |
| `--insecure` | `-k` | Не проверять TLS сертификат |

```bash
# все списки
./mikrotik-lists-manager list -H 192.168.1.1 -u admin

# по убыванию размера
./mikrotik-lists-manager list -H 192.168.1.1 -u admin --sort size

# только списки с "vpn" в имени
./mikrotik-lists-manager list -H 192.168.1.1 -u admin -F vpn

# все записи конкретного списка
./mikrotik-lists-manager list -H 192.168.1.1 -u admin -e vpn-routes
```

---

### `append` — добавление записей

Добавляет в список на роутере только те записи из файла, которых там ещё нет. Существующие записи не трогает и не обновляет.

```shell
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

```shell
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

```shell
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

```shell
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

Читает native `.list` файл и выполняет:
- удаление дублирующихся адресов и доменов
- удаление IP/CIDR которые полностью покрываются более широкой подсетью
- конвертацию `x.x.x.x/32` в `x.x.x.x`

По умолчанию выводит результат в stdout. С флагом `-w` перезаписывает файл.

```shell
./mikrotik-lists-manager optimize [file] [флаги]
```

| Флаг | Короткий | Описание |
|------|----------|----------|
| `--write` | `-w` | Перезаписать файл вместо вывода в stdout |

```bash
# посмотреть что будет изменено
./mikrotik-lists-manager optimize list.lst

# применить оптимизацию
./mikrotik-lists-manager optimize list.lst -w
```

---

### `fetch` — автозагрузка CIDR-диапазонов

Скачивает актуальные IPv4 CIDR-диапазоны из публичных источников и сохраняет в native `.lst` файл с секциями по провайдерам. Провайдеры загружаются параллельно.

Без флагов запускает интерактивный TUI для выбора провайдеров и сервисов.

```shell
./mikrotik-lists-manager fetch [флаги]
```

| Флаг | Короткий | Описание |
|------|----------|----------|
| `--output` | `-o` | Путь к выходному файлу (обязательный) |
| `--provider` | `-p` | Провайдеры: `-p cloudflare,google` или `-p cloudflare -p google` |
| `--asn` | `-A` | Произвольный ASN через RIPE STAT: `-A AS12345` или `-A 12345,67890` |
| `--all` | `-a` | Скачать все провайдеры без интерактивного выбора |
| `--format` | `-f` | Формат вывода: `native` (по умолчанию) или `mikrotik` (RSC скрипт) |
| `--merge` | `-m` | Обновить секции в существующем файле, не перезаписывая его целиком |
| `--timeout` | `-t` | Таймаут HTTP-запроса в секундах (по умолчанию 30) |

**Доступные провайдеры:**

| Провайдер | Slug | Источник |
|-----------|------|----------|
| Cloudflare | `cloudflare` | cloudflare.com/ips-v4 |
| Google | `google` | gstatic.com (goog.json + cloud.json) |
| AWS | `aws` | ip-ranges.amazonaws.com |
| Azure | `azure` | Microsoft Download Center |
| Fastly | `fastly` | api.fastly.com/public-ip-list |
| Akamai | `akamai` | techdocs.akamai.com |
| DigitalOcean | `digitalocean` | digitalocean.com/geo/google.csv |
| Hetzner | `hetzner` | RIPE STAT (AS24940) |
| OVH | `ovh` | RIPE STAT (AS16276) |
| Meta | `meta` | AS32934, AS63293 |
| Twitter / X | `twitter` | RIPE STAT (AS13414) |
| TikTok / ByteDance | `tiktok` | RIPE STAT (AS396986, AS138699) |
| Discord | `discord` | RIPE STAT (AS36459) |
| LinkedIn | `linkedin` | RIPE STAT (AS14413) |
| Pornhub / MindGeek | `pornhub` | RIPE STAT (AS55222, AS29789) |
| Netflix | `netflix` | RIPE STAT (AS2906) |
| Twitch | `twitch` | RIPE STAT (AS46489) |
| Steam / Valve | `steam` | RIPE STAT (AS32590) |
| Blizzard | `blizzard` | RIPE STAT (AS57976, AS209242) |
| Riot Games | `riot` | RIPE STAT (AS6507, AS26008) |
| Ubisoft | `ubisoft` | RIPE STAT (AS39561) |
| EA / Electronic Arts | `ea` | RIPE STAT (AS12128, AS14068) |
| Epic Games | `epic` | RIPE STAT (AS46562) |
| Roblox | `roblox` | RIPE STAT (AS22697) |
| Apple | `apple` | RIPE STAT (AS714, AS6185) |
| Yandex | `yandex` | RIPE STAT (AS13238) |
| VK | `vk` | RIPE STAT (AS47541, AS44507) |
| Telega (VK) | `telega` | RIPE STAT (AS203502) |
| Mail.ru | `mailru` | RIPE STAT (AS47764, AS57620) |
| Zoom | `zoom` | RIPE STAT (AS8100, AS21929) |
| Reddit | `reddit` | RIPE STAT (AS54009, AS22616) |
| Telegram | `telegram` | core.telegram.org/resources/cidr.txt |
| Tor | `tor` | check.torproject.org/torbulkexitlist |
| GitHub | `github` | api.github.com/meta (выбор сервисов) |
| Oracle Cloud | `oracle` | docs.oracle.com (выбор регионов) |

GitHub поддерживает выбор сервисов через `/`: `github/copilot`, `github/actions`, `github/web` и др.
Oracle поддерживает выбор регионов через `/`: `oracle/eu-frankfurt-1`, `oracle/us-ashburn-1` и др.

```bash
# интерактивный TUI-выбор
./mikrotik-lists-manager fetch -o ranges.lst

# все провайдеры сразу
./mikrotik-lists-manager fetch -a -o ranges.lst

# конкретные провайдеры
./mikrotik-lists-manager fetch -p cloudflare,telegram -o ranges.lst

# GitHub — только Copilot и Web
./mikrotik-lists-manager fetch -p github/copilot,github/web -o ranges.lst

# Oracle — конкретные регионы
./mikrotik-lists-manager fetch -p oracle/eu-frankfurt-1,oracle/us-ashburn-1 -o ranges.lst

# смешанно
./mikrotik-lists-manager fetch -p cloudflare,telegram,github/copilot -o ranges.lst

# произвольный ASN
./mikrotik-lists-manager fetch -A AS55222 -o pornhub.lst
./mikrotik-lists-manager fetch -A 12345,67890 -o custom.lst

# ASN вместе с провайдерами
./mikrotik-lists-manager fetch -A AS203502 -p telegram -o combined.lst

# вывод в формате MikroTik RSC скрипта
./mikrotik-lists-manager fetch -p cloudflare,telegram -f mikrotik -o ranges.rsc

# обновить только изменившиеся секции в существующем файле
./mikrotik-lists-manager fetch -p cloudflare,telegram -m -o ranges.lst
```

Если провайдер недоступен — выводится предупреждение, остальные продолжают скачиваться.

---

### `find` — поиск адреса на роутере

Ищет IP или CIDR во всех address-list на роутере. Находит точные совпадения, а также проверяет попадание IP в CIDR-записи и наоборот.

```shell
./mikrotik-lists-manager find <address> [флаги]
```

| Флаг | Короткий | Описание |
|------|----------|----------|
| `--host` | `-H` | Адрес роутера |
| `--user` | `-u` | Имя пользователя API |
| `--pass` | `-p` | Пароль |
| `--insecure` | `-k` | Не проверять TLS сертификат |

```bash
# найти конкретный IP (включая попадание в CIDR-записи)
./mikrotik-lists-manager find 8.8.8.8 -H 192.168.1.1 -u admin

# найти все записи, входящие в подсеть
./mikrotik-lists-manager find 8.8.0.0/16 -H 192.168.1.1 -u admin
```

---

### `backup` — резервное копирование

Сохраняет все статические address-list с роутера в папку — один файл на список.

```shell
./mikrotik-lists-manager backup [флаги]
```

| Флаг | Короткий | Описание |
|------|----------|----------|
| `--host` | `-H` | Адрес роутера |
| `--user` | `-u` | Имя пользователя API |
| `--pass` | `-p` | Пароль |
| `--output` | `-o` | Папка для сохранения файлов (по умолчанию `.`) |
| `--format` | `-f` | Формат: `native` (по умолчанию) или `mikrotik` |
| `--insecure` | `-k` | Не проверять TLS сертификат |

```bash
# сохранить все списки в папку ./backup
./mikrotik-lists-manager backup -H 192.168.1.1 -u admin -o ./backup

# в формате MikroTik RSC
./mikrotik-lists-manager backup -H 192.168.1.1 -u admin -o ./backup -f mikrotik
```

---

### `rename` — переименование списка

Переименовывает address-list на роутере, обновляя поле `list` у всех его записей через REST API.

```shell
./mikrotik-lists-manager rename <old-name> <new-name> [флаги]
```

| Флаг | Короткий | Описание |
|------|----------|----------|
| `--host` | `-H` | Адрес роутера |
| `--user` | `-u` | Имя пользователя API |
| `--pass` | `-p` | Пароль |
| `--insecure` | `-k` | Не проверять TLS сертификат |

```bash
./mikrotik-lists-manager rename vpn-old vpn-routes -H 192.168.1.1 -u admin
```

---

### `info` — информация о роутере

Подключается к роутеру и выводит информационный блок: модель, RouterOS, CPU, память, аптайм, прошивка RouterBoard.

```shell
./mikrotik-lists-manager info [флаги]
```

```bash
./mikrotik-lists-manager info -H 192.168.1.1 -u admin
```

---

### `completion` — автодополнение оболочки

Генерирует скрипт автодополнения команд и флагов для популярных оболочек.

```shell
./mikrotik-lists-manager completion [bash|zsh|fish|powershell]
```

```bash
# Bash
./mikrotik-lists-manager completion bash > /etc/bash_completion.d/mikrotik-lists-manager

# Zsh
./mikrotik-lists-manager completion zsh > "${fpath[1]}/_mikrotik-lists-manager"

# Fish
./mikrotik-lists-manager completion fish > ~/.config/fish/completions/mikrotik-lists-manager.fish
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

## ⚙️ Конфигурационный файл

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
./mikrotik-lists-manager fetch -o ranges.lst
```

---

## 🌍 Переменные окружения

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

## 🔍 Сравнение команд

| Команда | Добавляет | Удаляет | Обновляет | Трогает только из файла |
|---------|-----------|---------|-----------|------------------------|
| `sync` | ✓ | ✓ | ✓ | — (полная синхронизация) |
| `append` | ✓ | — | — | ✓ |
| `remove` | — | ✓ | — | ✓ |

---

## 🔧 Настройка MikroTik

Для работы REST API нужен пользователь с правами на чтение/запись firewall:

```routeros
/user group add name=api-sync policy=read,write,api,rest-api
/user add name=sync group=api-sync password=yourpassword
```

REST API включён по умолчанию в RouterOS 7. Проверить активные сервисы:

```routeros
/ip service print
```

Должен быть активен `www-ssl` (порт 443) или `www` (порт 80).

---

## 📜 Лицензия

[MIT](LICENSE)
