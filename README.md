# 🛠 mlm

CLI утилита для управления firewall address-list на MikroTik из локального файла или URL.

Подключается к роутеру через REST API (RouterOS 7+), сравнивает текущее состояние списка с файлом и приводит его к нужному виду — добавляет, удаляет, обновляет и включает/отключает записи. Динамические записи (`dynamic=true`) не трогает.

## Содержание

- [Установка](#установка)
- [Быстрый старт](#быстрый-старт)
- [Формат файла и источники данных](#формат-файла-и-источники-данных)
- [Несколько списков](#несколько-списков)
- [Поддержка прокси](#поддержка-прокси)
- [Команды](#команды)
  - [sync](#sync--полная-синхронизация)
  - [diff](#diff--сравнение-списков-оффлайн-и-с-роутером)
  - [validate](#validate--валидация-и-проверка-списков)
  - [snapshot / rollback](#snapshot--rollback--снимки-и-быстрый-откат)
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

### 1. Через Go пакетный менеджер (`go install`)

Утилиту можно установить в систему одной командой через Go:

```bash
go install github.com/D4n13l3k00/mikrotik-lists-manager/cmd/mlm@latest
```

Бинарник `mlm` устанавливается в `$GOPATH/bin` (по умолчанию `~/go/bin`), который обычно уже находится в системной переменной `PATH`.

---

### 2. Готовые бинарные файлы

Скачать готовый бинарник `mlm` для своей ОС и архитектуры (Linux, Windows, macOS, FreeBSD, ARM) со [страницы релизов](https://github.com/D4n13l3k00/mikrotik-lists-manager/releases).

---

### 3. Сборка из исходников

```bash
git clone https://github.com/D4n13l3k00/mikrotik-lists-manager
cd mikrotik-lists-manager

# Собрать и установить бинарник mlm в $GOPATH/bin:
go install ./cmd/mlm

# Или скомпилировать в текущую папку:
go build -o mlm ./cmd/mlm
```

**Требования:** RouterOS 7.x (REST API), Go 1.23+ для сборки из исходников.

---

## 🚀 Быстрый старт

```bash
# проверить синтаксис, неканонические маски и дубликаты
mlm validate list.lst

# сравнить два списка оффлайн перед применением
mlm diff base.lst https://example.com/new.lst

# синхронизировать список напрямую по URL (с автосозданием точки восстановления)
mlm sync https://example.com/vpn.lst -H 192.168.1.1 -u admin -l vpn-routes

# выполнить откат списка при необходимости
mlm rollback -l vpn-routes -H 192.168.1.1 -u admin

# посмотреть все списки на роутере
mlm list -H 192.168.1.1 -u admin

# скачать актуальные CIDR от провайдеров
mlm fetch -o ranges.lst
```

Пароль будет запрошен интерактивно если не передан флагом `-p`.

---

## 📄 Формат файла и источники данных

### Источники данных (`[file|url]`)

Во всех командах работы со списками (`sync`, `append`, `remove`, `optimize`, `diff`, `validate`) в качестве источника можно передавать:
1. **Локальный файл**: `vpn.list`, `./lists/hosts.txt`
2. **Прямой HTTP / HTTPS URL**: `https://raw.githubusercontent.com/.../list.lst`
3. **Стандартный ввод (stdin)**: `-` (например: `curl -s https://... | mlm sync -`)

---

### Поддерживаемые форматы

#### 1. Native (`.list`)

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

#### 2. MikroTik export (`.rsc`)

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
mlm sync list.lst -l vpn,blocked
mlm sync list.lst -l vpn -l blocked
mlm disable -a -l vpn,blocked,whitelist
```

---

## 🌐 Поддержка прокси

Утилита поддерживает работу через прокси-серверы для **внешних сетевых операций и DNS**:
- Загрузка удаленных списков по URL (`sync`, `diff`, `validate`, `append`, `remove`, `optimize`).
- Скачивание диапазонов провайдеров (`fetch`).
- Локальный резолвинг доменов в IP (`--resolve-domains`) через туннелирование DNS-over-TCP (RFC 1035 / RFC 7766) через прокси-диалер.

> **Примечание:** Подключение к REST API самого MikroTik всегда выполняется **напрямую (без прокси)**, так как роутер обычно находится в локальной или доверенной сети.

### Поддерживаемые протоколы

| Схема | Описание |
|---|---|
| `socks5://[user:pass@]host:port` | SOCKS5 с локальным DNS-резолвингом адреса прокси |
| `socks5h://[user:pass@]host:port` | SOCKS5 с удаленным резолвингом доменных имен через прокси |
| `http://[user:pass@]host:port` | HTTP CONNECT туннелирование (поддерживается Basic Auth) |
| `https://[user:pass@]host:port` | HTTPS CONNECT туннелирование |

### Способы указания

1. **Глобальный флаг CLI** `--proxy`:
   ```bash
   mlm sync https://example.com/vpn.lst --proxy socks5://127.0.0.1:1080 -l vpn
   mlm diff list1.lst list2.lst --proxy http://10.0.0.1:8080 --resolve-domains
   mlm fetch -p cloudflare -o cf.lst --proxy socks5h://127.0.0.1:9050
   ```

2. **Переменная окружения** `MT_PROXY`:
   ```bash
   export MT_PROXY="socks5://127.0.0.1:1080"
   mlm sync vpn.lst -l vpn
   ```

3. **В файле конфигурации** (глобально или внутри профилей роутеров):
   ```yaml
   # глобальный прокси по умолчанию
   proxy: "socks5://127.0.0.1:1080"

   profiles:
     office:
       host: "10.0.0.1"
       user: "admin"
       list: "office-routes"
       proxy: "http://proxy.corp:8080"
   ```

Приоритет: `--proxy` флаг > `MT_PROXY` > профиль `proxy` > глобальный `proxy` в конфиге.

---

## 📋 Команды

### `sync` — полная синхронизация

Читает файл или URL, получает текущий список с роутера, вычисляет diff и приводит список на роутере к точному состоянию источника: добавляет отсутствующие, удаляет лишние, обновляет комментарии и состояние `disabled`.

Перед применением изменений автоматически создается локальный снимок (snapshot) для безопасного отката (отключается флагом `--no-snapshot`).

Если запись в источнике без `!`, но на роутере она `disabled=true` — включит обратно.
При 10+ изменениях показывает прогресс-бар. Флаг `-v` включает построчный вывод вместе с баром.
API-запросы выполняются параллельно (флаг `-c`, по умолчанию 5). При подключении выводится баннер роутера.

```shell
mlm sync [file|url] [флаги]
```

| Флаг | Короткий | Описание |
|------|----------|----------|
| `--host` | `-H` | Адрес роутера: `192.168.1.1`, `http://10.0.0.1`, `https://host:8443` |
| `--user` | `-u` | Имя пользователя API |
| `--pass` | `-p` | Пароль (если не задан — запросит интерактивно) |
| `--list` | `-l` | Имя address-list, можно несколько: `-l a,b` или `-l a -l b` |
| `--format` | `-f` | Формат файла: `auto`, `native`, `mikrotik` (по умолчанию `auto`) |
| `--dry-run` | `-n` | Показать изменения без применения |
| `--force` | `-y` | Пропустить подтверждение при массовом удалении (>50 записей или >50% списка) |
| `--resolve-domains` | | Разрешать доменные имена в IP-адреса через DNS |
| `--dns` | | Пользовательский DNS-сервер для резолвинга (например: `1.1.1.1:53`) |
| `--no-snapshot` | | Не создавать автоматический снимок перед применением |
| `--verbose` | `-v` | Выводить каждую запись даже при прогресс-баре |
| `--concurrency` | `-c` | Число параллельных запросов к API (по умолчанию 5, 0 = последовательно) |
| `--watch` | `-w` | Следить за файлом и пересинхронизировать при изменении |
| `--watch-interval` | | Интервал проверки файла в секундах (по умолчанию 3, с `--watch`) |
| `--insecure` | `-k` | Не проверять TLS сертификат |
| `--proxy` | | URL прокси-сервера (`socks5://`, `socks5h://`, `http://`, `https://`) |

```bash
# обычная синхронизация файла
mlm sync vpn.list -H 192.168.1.1 -u admin -l vpn-routes

# прямая синхронизация по HTTP/HTTPS URL
mlm sync https://raw.githubusercontent.com/.../vpn.list -H 192.168.1.1 -u admin -l vpn-routes

# синхронизация с локальным резолвингом доменов в IP
mlm sync domains.txt -H 192.168.1.1 -u admin -l vpn-routes --resolve-domains

# dry-run — только посмотреть diff
mlm sync vpn.list -H 192.168.1.1 -u admin -l vpn-routes -n

# синхронизировать в несколько списков (обрабатываются последовательно)
mlm sync vpn.list -H 192.168.1.1 -u admin -l vpn,blocked

# из stdin
cat vpn.list | mlm sync - -H 192.168.1.1 -u admin -l vpn-routes

# без подтверждения при массовом удалении
mlm sync vpn.list -H 192.168.1.1 -u admin -l vpn-routes -y

# следить за файлом и синхронизировать при каждом изменении
mlm sync vpn.list -H 192.168.1.1 -u admin -l vpn-routes -w
```

---

### `diff` — сравнение списков (оффлайн и с роутером)

Позволяет наглядно увидеть разницу между двумя источниками:
1. **Оффлайн-сравнение (2 аргумента)**: сравнивает два файла, URL или stdin без подключения к роутеру. Первый аргумент считается исходным (базовым), второй — целевым.
2. **Сравнение со списком на роутере (1 аргумент + `-l`)**: подключается к MikroTik, считывает текущий address-list и сравнивает его с локальным файлом или URL.

Выводит привычный unified-diff:
- `+` (зелёный) — запись будет добавлена
- `−` (красный) — запись будет удалена
- `~` (жёлтый) — изменился комментарий или статус активности (`disabled`)

```shell
mlm diff <source1> [source2] [флаги]
```

| Флаг | Короткий | Описание |
|------|----------|----------|
| `--host` | `-H` | Адрес роутера (для режима сравнения с роутером) |
| `--user` | `-u` | Имя пользователя API |
| `--pass` | `-p` | Пароль |
| `--list` | `-l` | Имя address-list на роутере |
| `--format` | `-f` | Формат файлов: `auto`, `native`, `mikrotik` |
| `--resolve-domains` | | Разрешать доменные имена в IP-адреса через DNS |
| `--dns` | | Пользовательский DNS-сервер для резолвинга |
| `--json` | | Вывод изменений и сводки в формате JSON |
| `--insecure` | `-k` | Не проверять TLS сертификат |

```bash
# оффлайн сравнение двух локальных файлов
mlm diff base.lst updated.lst

# оффлайн сравнение локального файла с удаленным URL
mlm diff local.lst https://example.com/remote.lst

# сравнение файла с текущим состоянием списка на роутере
mlm diff vpn.lst -l vpn-routes -H 192.168.1.1 -u admin

# сравнение с локальным DNS-резолвингом доменов
mlm diff domains.txt -l vpn-routes --resolve-domains

# вывод diff в JSON для автоматизации
mlm diff base.lst new.lst --json
```

---

### `validate` — валидация и проверка списков

Инструмент проверки и линтинга списков адресов перед применением или коммитом в репозиторий.
Выполняет глубокий анализ:
- **Синтаксис адресов**: проверка корректности IPv4, IPv6, CIDR и доменных имён.
- **Неканонические подсети**: предупреждение о ненулевых битах хоста в CIDR (например `192.168.1.5/24` вместо `192.168.1.0/24`).
- **Дубликаты**: поиск повторяющихся адресов внутри списка.
- **Поглощённые подсети (shadowed subnets)**: обнаружение IP или подсетей, полностью перекрытых более широкими сетями в том же файле (например `10.0.1.0/24` внутри `10.0.0.0/16`).
- **Метрики списка**: подсчёт хостов, подсетей, доменов, отключённых записей.

```shell
mlm validate <file|url> [флаги]
```

| Флаг | Короткий | Описание |
|------|----------|----------|
| `--strict` | | Считать предупреждения (warnings) ошибками (exit code 1) — для CI/CD |
| `--json` | | Вывод структурированного отчёта в формате JSON |
| `--format` | `-f` | Формат файла: `auto`, `native`, `mikrotik` |

```bash
# обычная проверка с выводом сводки в терминал
mlm validate list.lst

# проверка удаленного списка по ссылке
mlm validate https://example.com/blocked.txt

# строгий режим (любое предупреждение возвращает ненулевой код возврата)
mlm validate list.lst --strict

# машиночитаемый вывод отчёта для проверок в скриптах
mlm validate list.lst --json
```

---

### `snapshot` / `rollback` — снимки и быстрый откат

Система создания локальных точек восстановления списков адресов и мгновенного возврата к предыдущему состоянию.

- При выполнении `sync` снимок создаётся **автоматически** перед внесением изменений.
- Снимки сохраняются локально в системной директории пользователя (`~/.config/mlm/snapshots/` или `%APPDATA%\mlm\snapshots\`).
- Для каждого списка роутера автоматически сохраняются до 10 последних снимков (старые ротируются).

#### Управление снимками (`snapshot`)

```shell
# просмотр истории снимков для списка
mlm snapshot list -l vpn-routes -H 192.168.1.1 -u admin

# просмотр истории снимков в формате JSON
mlm snapshot list -l vpn-routes --json

# создание снимка вручную перед экспериментами
mlm snapshot create -l vpn-routes -H 192.168.1.1 -u admin

# удаление снимка по ID
mlm snapshot delete 20260916-120000_abcd -l vpn-routes -H 192.168.1.1
```

#### Восстановление состояния (`rollback`)

Команда `rollback` восстанавливает состояние списка на роутере из снимка. По умолчанию восстанавливает самый свежий снимок (`latest`).

```shell
mlm rollback [флаги]
```

| Флаг | Короткий | Описание |
|------|----------|----------|
| `--host` | `-H` | Адрес роутера |
| `--user` | `-u` | Имя пользователя API |
| `--pass` | `-p` | Пароль |
| `--list` | `-l` | Имя address-list |
| `--id` | | ID конкретного снимка (по умолчанию `latest`) |
| `--dry-run` | `-n` | Показать планируемые изменения отката без применения |
| `--force` | `-y` | Пропустить подтверждение при массовом удалении записей |
| `--concurrency` | `-c` | Число параллельных запросов к API |
| `--verbose` | `-v` | Выводить каждую запись подробно |
| `--insecure` | `-k` | Не проверять TLS сертификат |

```bash
# предварительный просмотр изменений при откате к последнему снимку
mlm rollback -l vpn-routes -H 192.168.1.1 -u admin -n

# откат списка к последней точке восстановления
mlm rollback -l vpn-routes -H 192.168.1.1 -u admin

# откат к конкретному снимку из истории
mlm rollback -l vpn-routes --id 20260916-120000_abcd -H 192.168.1.1 -u admin
```

---

### `list` — просмотр списков на роутере

Показывает все address-list на роутере с количеством записей и сколько из них отключено. При выводе записей конкретного списка (`-e`) они сортируются натуральным образом по IP/CIDR.

```shell
mlm list [флаги]
```

| Флаг | Короткий | Описание |
|------|----------|----------|
| `--host` | `-H` | Адрес роутера |
| `--user` | `-u` | Имя пользователя API |
| `--pass` | `-p` | Пароль |
| `--entries` | `-e` | Показать все записи конкретного списка |
| `--sort` | | Сортировка: `name` (по умолчанию) или `size` (по количеству записей) |
| `--filter` | `-F` | Фильтр по имени списка (подстрока, без учёта регистра) |
| `--json` | | Машиночитаемый вывод в формате JSON |
| `--insecure` | `-k` | Не проверять TLS сертификат |

```bash
# все списки
mlm list -H 192.168.1.1 -u admin

# в формате JSON
mlm list -H 192.168.1.1 -u admin --json

# по убыванию размера
mlm list -H 192.168.1.1 -u admin --sort size

# только списки с "vpn" в имени
mlm list -H 192.168.1.1 -u admin -F vpn

# все записи конкретного списка
mlm list -H 192.168.1.1 -u admin -e vpn-routes

# записи списка в формате JSON
mlm list -H 192.168.1.1 -u admin -e vpn-routes --json
```

---

### `append` — добавление записей

Добавляет в список на роутере только те записи из файла или URL, которых там ещё нет. Существующие записи не трогает и не обновляет.

```shell
mlm append [file|url] [флаги]
```

| Флаг | Короткий | Описание |
|------|----------|----------|
| `--host` | `-H` | Адрес роутера |
| `--user` | `-u` | Имя пользователя API |
| `--pass` | `-p` | Пароль |
| `--list` | `-l` | Имя address-list, можно несколько |
| `--format` | `-f` | Формат файла: `auto`, `native`, `mikrotik` |
| `--dry-run` | `-n` | Показать изменения без применения |
| `--resolve-domains` | | Разрешать доменные имена в IP-адреса через DNS |
| `--dns` | | Пользовательский DNS-сервер для резолвинга |
| `--concurrency` | `-c` | Число параллельных запросов к API (по умолчанию 5, 0 = последовательно) |
| `--insecure` | `-k` | Не проверять TLS сертификат |

```bash
# добавить новые записи из локального файла, дубли пропустить
mlm append extra.list -H 192.168.1.1 -u admin -l vpn-routes

# добавить записи напрямую по URL
mlm append https://example.com/new-ips.txt -H 192.168.1.1 -u admin -l vpn-routes

# добавить домены с предварительным резолвингом в IP
mlm append domains.lst -H 192.168.1.1 -u admin -l vpn-routes --resolve-domains

# добавить в несколько списков сразу
mlm append extra.list -H 192.168.1.1 -u admin -l vpn,blocked
```

---

### `remove` — удаление записей по файлу

Удаляет с роутера только те записи, которые перечислены в файле. Остальные записи в списке не трогает. Если адрес из файла не найден на роутере — выводит предупреждение.

```shell
mlm remove [file] [флаги]
```

| Флаг | Короткий | Описание |
|------|----------|----------|
| `--host` | `-H` | Адрес роутера |
| `--user` | `-u` | Имя пользователя API |
| `--pass` | `-p` | Пароль |
| `--list` | `-l` | Имя address-list, можно несколько |
| `--format` | `-f` | Формат файла: `auto`, `native`, `mikrotik` |
| `--dry-run` | `-n` | Показать изменения без применения |
| `--concurrency` | `-c` | Число параллельных запросов к API (по умолчанию 5, 0 = последовательно) |
| `--insecure` | `-k` | Не проверять TLS сертификат |

```bash
# удалить записи из файла, остальные оставить
mlm remove telegram.list -H 192.168.1.1 -u admin -l vpn-routes

# посмотреть что удалится
mlm remove telegram.list -H 192.168.1.1 -u admin -l vpn-routes -n
```

---

### `export` — экспорт

Получает текущий список с роутера и выводит его в stdout или файл. При нескольких списках и `-o` все списки записываются в один файл подряд.

```shell
mlm export [флаги]
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
mlm export -H 192.168.1.1 -u admin -l vpn-routes

# сохранить несколько списков в один файл
mlm export -H 192.168.1.1 -u admin -l vpn,blocked -o backup.list

# сохранить в формате MikroTik export
mlm export -H 192.168.1.1 -u admin -l vpn-routes -f mikrotik -o backup.rsc
```

---

### `enable` / `disable` — включение и отключение

Включает или отключает конкретные записи или весь список на роутере.
**Не изменяет локальный файл** — только состояние на роутере.

```shell
mlm enable [адрес...] [флаги]
mlm disable [адрес...] [флаги]
```

| Флаг | Короткий | Описание |
|------|----------|----------|
| `--host` | `-H` | Адрес роутера |
| `--user` | `-u` | Имя пользователя API |
| `--pass` | `-p` | Пароль |
| `--list` | `-l` | Имя address-list, можно несколько |
| `--all` | `-a` | Применить ко всем записям списка |
| `--concurrency` | `-c` | Число параллельных запросов к API (по умолчанию 5, 0 = последовательно) |
| `--insecure` | `-k` | Не проверять TLS сертификат |

```bash
# отключить конкретные записи
mlm disable 8.8.8.8 1.1.1.1 -H 192.168.1.1 -u admin -l vpn-routes

# включить конкретные записи
mlm enable 8.8.8.8 -H 192.168.1.1 -u admin -l vpn-routes

# отключить несколько списков целиком
mlm disable -a -H 192.168.1.1 -u admin -l vpn,blocked

# включить весь список
mlm enable -a -H 192.168.1.1 -u admin -l vpn-routes
```

---

### `optimize` — оптимизация файла

Читает native `.list` файл или URL и выполняет:
- удаление дублирующихся адресов и доменов
- удаление IP/CIDR которые полностью покрываются более широкой подсетью
- конвертацию `x.x.x.x/32` в `x.x.x.x`

По умолчанию выводит результат в stdout. С флагом `-w` перезаписывает локальный файл.

```shell
mlm optimize [file|url] [флаги]
```

| Флаг | Короткий | Описание |
|------|----------|----------|
| `--write` | `-w` | Перезаписать файл вместо вывода в stdout (только для локальных файлов) |

```bash
# посмотреть что будет изменено
mlm optimize list.lst

# скачать удаленный список, оптимизировать и сохранить в файл
mlm optimize https://example.com/huge.lst > list.lst

# применить оптимизацию с перезаписью файла
mlm optimize list.lst -w
```

---

### `fetch` — автозагрузка CIDR-диапазонов

Скачивает актуальные IPv4 CIDR-диапазоны из публичных источников и сохраняет в native `.lst` файл с секциями по провайдерам. Провайдеры загружаются параллельно.

Без флагов запускает интерактивный TUI для выбора провайдеров и сервисов.

```shell
mlm fetch [флаги]
```

| Флаг | Короткий | Описание |
|------|----------|----------|
| `--output` | `-o` | Путь к выходному файлу (обязательный) |
| `--provider` | `-p` | Провайдеры: `-p cloudflare,google` или `-p cloudflare -p google` |
| `--asn` | `-A` | Произвольный ASN через RIPE STAT: `-A AS12345` или `-A 12345,67890` |
| `--all` | `-a` | Скачать все провайдеры без интерактивного выбора |
| `--concurrency` | `-c` | Число параллельных загрузок (по умолчанию 6) |
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
mlm fetch -o ranges.lst

# все провайдеры сразу
mlm fetch -a -o ranges.lst

# конкретные провайдеры
mlm fetch -p cloudflare,telegram -o ranges.lst

# GitHub — только Copilot и Web
mlm fetch -p github/copilot,github/web -o ranges.lst

# Oracle — конкретные регионы
mlm fetch -p oracle/eu-frankfurt-1,oracle/us-ashburn-1 -o ranges.lst

# смешанно
mlm fetch -p cloudflare,telegram,github/copilot -o ranges.lst

# произвольный ASN
mlm fetch -A AS55222 -o pornhub.lst
mlm fetch -A 12345,67890 -o custom.lst

# ASN вместе с провайдерами
mlm fetch -A AS203502 -p telegram -o combined.lst

# вывод в формате MikroTik RSC скрипта
mlm fetch -p cloudflare,telegram -f mikrotik -o ranges.rsc

# обновить только изменившиеся секции в существующем файле
mlm fetch -p cloudflare,telegram -m -o ranges.lst
```

Если провайдер недоступен — выводится предупреждение, остальные продолжают скачиваться.

---

### `find` — поиск адреса на роутере

Ищет IP или CIDR во всех address-list на роутере. Находит точные совпадения (с канонической нормализацией), а также проверяет попадание IP в CIDR-записи и вложенность подсетей (CIDR в CIDR).

```shell
mlm find <address> [флаги]
```

| Флаг | Короткий | Описание |
|------|----------|----------|
| `--host` | `-H` | Адрес роутера |
| `--user` | `-u` | Имя пользователя API |
| `--pass` | `-p` | Пароль |
| `--json` | | Вывод в формате JSON |
| `--insecure` | `-k` | Не проверять TLS сертификат |

```bash
# найти конкретный IP (включая попадание в CIDR-записи)
mlm find 8.8.8.8 -H 192.168.1.1 -u admin

# найти все записи, входящие в подсеть или содержащие её
mlm find 8.8.0.0/16 -H 192.168.1.1 -u admin

# вывод в JSON
mlm find 1.1.1.1 -H 192.168.1.1 -u admin --json
```

---

### `backup` — резервное копирование

Сохраняет все статические address-list с роутера в папку — один файл на список.

```shell
mlm backup [флаги]
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
mlm backup -H 192.168.1.1 -u admin -o ./backup

# в формате MikroTik RSC
mlm backup -H 192.168.1.1 -u admin -o ./backup -f mikrotik
```

---

### `rename` — переименование списка

Переименовывает address-list на роутере, обновляя поле `list` у всех его записей через REST API параллельно с прогресс-баром.

```shell
mlm rename <old-name> <new-name> [флаги]
```

| Флаг | Короткий | Описание |
|------|----------|----------|
| `--host` | `-H` | Адрес роутера |
| `--user` | `-u` | Имя пользователя API |
| `--pass` | `-p` | Пароль |
| `--concurrency` | `-c` | Число параллельных запросов к API (по умолчанию 5, 0 = последовательно) |
| `--insecure` | `-k` | Не проверять TLS сертификат |

```bash
mlm rename vpn-old vpn-routes -H 192.168.1.1 -u admin
```

---

### `info` — информация о роутере

Подключается к роутеру и выводит информационный блок: модель, RouterOS, CPU, память, аптайм, прошивка RouterBoard. Поддерживает вывод в формате JSON.

```shell
mlm info [флаги]
```

| Флаг | Короткий | Описание |
|------|----------|----------|
| `--host` | `-H` | Адрес роутера |
| `--user` | `-u` | Имя пользователя API |
| `--pass` | `-p` | Пароль |
| `--json` | | Вывод в формате JSON |
| `--insecure` | `-k` | Не проверять TLS сертификат |

```bash
mlm info -H 192.168.1.1 -u admin

# вывод в JSON
mlm info -H 192.168.1.1 -u admin --json
```

---

### `completion` — автодополнение оболочки

Генерирует скрипт автодополнения команд и флагов для популярных оболочек.

```shell
mlm completion [bash|zsh|fish|powershell]
```

```bash
# Bash
mlm completion bash > /etc/bash_completion.d/mlm

# Zsh
mlm completion zsh > "${fpath[1]}/_mlm"

# Fish
mlm completion fish > ~/.config/fish/completions/mlm.fish
```

---

### `config` — управление конфигом

#### `config init`

Создаёт шаблон конфигурационного файла. По умолчанию создаёт `.mlm.yaml` в текущей директории. С флагом `-g` (`--global`) создаёт конфиг в глобальной директории пользователя.

```bash
# в текущей директории
mlm config init

# в глобальной пользовательской директории (~/.config/mlm/config.yaml или %APPDATA%)
mlm config init -g

# по произвольному пути
mlm config init --config /etc/vpn/config.yaml
```

#### `config show`

Показывает итоговую конфигурацию с учётом файла, активного профиля и переменных окружения. Пароль маскируется.

```bash
mlm config show

# показать параметры конкретного профиля
mlm config show -P office
```

---

## ⚙️ Конфигурационный файл

### Автопоиск конфига

Если путь к конфигу не передан явно через `--config`, утилита ищет его по следующей цепочке:
1. Текущая директория: `./.mlm.yaml` (с обратной поддержкой `.mikrotik-lists-manager.yaml`)
2. Глобальная директория пользователя: `~/.config/mlm/config.yaml` (Linux/macOS) или `%APPDATA%\mlm\config.yaml` (Windows)
3. Домашняя директория пользователя: `~/.mlm.yaml`

Приоритет для каждого параметра: **CLI флаг > переменная окружения > выбранный профиль > глобальные настройки конфига > значения по умолчанию**.

### Профили роутеров (Profiles)

Конфиг поддерживает несколько профилей роутеров (например, домашний и офисный роутер). Для переключения используется флаг `-P` (`--profile`) или переменная окружения `MT_PROFILE`.

```yaml
# .mlm.yaml

# Профиль по умолчанию (если не указан -P)
default_profile: "home"

# Глобальные настройки (fallback для параметров, не заданных в профиле)
user: "admin"
pass: ""            # лучше оставить пустым — спросит интерактивно
insecure: false
default_format: auto
proxy: "socks5://127.0.0.1:1080" # socks5, socks5h, http, https

# Профили роутеров
profiles:
  home:
    host: "192.168.1.1"
    user: "admin"
    list: "vpn-routes"
  office:
    host: "10.10.0.1:8443"
    user: "network-admin"
    list: "office-vpn"
    insecure: true
    proxy: "http://proxy.corp:8080"
```

Использование профилей:

```bash
# использовать профиль по умолчанию (home)
mlm list

# использовать профиль office
mlm list -P office
mlm sync vpn.list -P office
```

После создания конфига команды становятся короче:

```bash
mlm sync list.lst -n
mlm sync list.lst
mlm list
mlm append extra.list
mlm remove telegram.list -n
mlm export -o backup.list
mlm disable -a
mlm fetch -o ranges.lst
```

---

## 🌍 Переменные окружения

| Переменная | Флаг |
|------------|------|
| `MT_HOST` | `-H` / `--host` |
| `MT_USER` | `-u` / `--user` |
| `MT_PASS` | `-p` / `--pass` |
| `MT_LIST` | `-l` / `--list` |
| `MT_PROFILE` | `-P` / `--profile` |
| `MT_PROXY` | `--proxy` |

```bash
export MT_HOST=192.168.1.1
export MT_USER=admin
export MT_LIST=vpn-routes

mlm sync list.lst -n
mlm list
mlm disable -a
```

---

## 🔍 Сравнение команд работы со списками

| Команда | Назначение | Добавляет | Удаляет | Обновляет | Требует роутер |
|---|---|:---:|:---:|:---:|:---:|
| `sync` | Полная синхронизация со снимком перед применением | ✓ | ✓ | ✓ | ✓ |
| `rollback` | Быстрый откат списка к точке восстановления | ✓ | ✓ | ✓ | ✓ |
| `diff` | Сравнение двух списков оффлайн или со списком на роутере | — | — | — | Опционально |
| `validate` | Проверка синтаксиса, некорректных масок и дубликатов | — | — | — | ✗ |
| `append` | Добавление недостающих записей без удаления | ✓ | — | — | ✓ |
| `remove` | Точечное удаление записей по файлу/URL | — | ✓ | — | ✓ |
| `optimize` | Локальная оптимизация и сжатие подсетей | — | — | — | ✗ |
| `export` | Экспорт списка в файл или stdout | — | — | — | ✓ |
| `backup` | Резервное копирование всех списков в файлы | — | — | — | ✓ |

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
