# Архитектура Liapoldus Frontend Constructor

## 1. Назначение

Сервис предназначен для создания и публикации frontend-сайтов из управляемых компонентов.

Сервис предоставляет:

- конструктор страниц;
- React-компоненты;
- маршрутизацию;
- темы;
- контент и локализацию;
- статические Assets;
- простые формы;
- configuration API и server-only endpoints;
- интеграцию с внешними системами через плагины;
- сборку frontend через esbuild (встроен в Go-бинарник, без Node);
- версионирование;
- Development и Production окружения;
- размещение нескольких независимых сайтов на одном backend.

Сервис не является API Gateway. Его backend-функциональность предназначена исключительно для обеспечения работы создаваемого frontend.

---

# 2. Основные понятия

| Объект              | Назначение                                         |
| ------------------- | -------------------------------------------------- |
| Site                | Независимый сайт                                   |
| Route               | Правило навигации                                  |
| Page                | Содержимое страницы                                |
| ComponentDefinition | Определение React-компонента                         |
| ComponentInstance   | Использование компонента на странице               |
| Binding             | Связь свойства компонента с данными                |
| Content             | Данные, используемые компонентом                   |
| ContentTranslation  | Локализованная версия Content                      |
| Asset               | Статический ресурс                                 |
| Theme               | Набор стилей и связанных ресурсов                  |
| Operation           | Абстрактная операция получения или отправки данных |
| Provider            | Реализация Operation                               |
| Endpoint            | Backend endpoint, предоставляемый frontend         |
| Plugin              | Расширение системы                                 |
| Version             | Версия отдельного объекта                          |
| Snapshot            | Снимок состояния сайта                             |
| Build               | Результат сборки Snapshot                          |
| Environment         | Окружение сайта                                    |

---

# 3. Site

`Site` является изолированным контейнером конфигурации сайта.

Site содержит:

- Routes;
- Pages;
- ComponentDefinitions;
- Content;
- ContentTranslations;
- Assets;
- Themes;
- Operations;
- Endpoints;
- Plugins;
- Versions.

Один backend может обслуживать множество Sites.

Site выбирается сервером на основании конфигурации.

---

# 4. Route

`Route` отвечает только за навигацию.

Route не является Page.

Route содержит:

- matcher;
- priority;
- navigation action.

Matcher поддерживает полные регулярные выражения.

Основные действия:

| Action     | Назначение                |
| ---------- | ------------------------- |
| RenderPage | Отобразить Page           |
| Redirect   | Выполнить перенаправление |

Пример:

```text
/articles/[0-9]+ → ArticlePage
/old → /new
```

Route не отвечает за backend-запросы.

---

# 5. Page

`Page` представляет содержимое страницы.

Page содержит корневой `ComponentInstance`.

Компоненты образуют дерево.

Page может использовать Layout, который является обычным ComponentDefinition специального типа.

---

# 6. ComponentDefinition

`ComponentDefinition` представляет управляемый системой React-компонент.

Определение содержит:

- имя;
- тип;
- источник `.tsx`;
- metadata;
- schema;
- ограничения использования.

Файл `.tsx` является реализацией компонента.

Metadata и schema хранятся отдельно от `.tsx`.

Пример:

```text
ComponentDefinition
    name: Card
    source: ./Card.tsx
    allowChildren: true
```

Компонент может использоваться повторно.

---

# 7. ComponentInstance

`ComponentInstance` представляет конкретное использование ComponentDefinition.

Содержит:

- ссылку на ComponentDefinition;
- props;
- bindings;
- дочерние ComponentInstances.

ComponentDefinition описывает компонент.

ComponentInstance описывает его использование.

---

# 8. Component Types

Тип компонента определяет его назначение.

Базовые типы могут включать:

| Type      | Назначение                   |
| --------- | ---------------------------- |
| Component | Обычный компонент            |
| Layout    | Компонент-контейнер страницы |
| Form      | Компонент формы              |

Система должна позволять добавлять новые типы через расширения.

---

# 9. Component Schema

Schema компонента определяет структуру данных, которую компонент ожидает получить.

Например:

```text
ArticleCard

title: string
description: string
image: Asset
```

Schema принадлежит ComponentDefinition.

Content не определяет собственную структуру.

---

# 10. Content

`Content` содержит данные, предназначенные для конкретного ComponentDefinition.

Данные хранятся в BSON.

Пример:

```text
{
    "title": "Hello",
    "description": "Description",
    "image": {
        "assetId": "..."
    }
}
```

Content не должен быть привязан к внешней системе.

Content является локальными данными сайта.

---

# 11. Content Translation

Локализация Content является отдельным объектом.

Один Content может иметь несколько ContentTranslation.

Каждая translation относится к одному языку.

```text
Content
    ├── Translation: ru
    ├── Translation: en
    └── Translation: de
```

Локализация относится к данным Content, а не к React-компоненту.

---

# 12. Asset

`Asset` представляет статический ресурс.

Поддерживаемые типы включают:

- изображения;
- SVG;
- шрифты;
- видео;
- документы;
- другие статические ресурсы.

Content хранит ссылку на Asset, а не URL.

URL или способ получения Asset определяется runtime.

Asset может использоваться несколькими объектами.

---

# 13. Theme

`Theme` представляет набор стилей и связанных ресурсов.

Theme может содержать:

- CSS;
- CSS variables;
- шрифты;
- дополнительные Assets;
- metadata.

На один объект назначается одна Theme.

Темы не наследуются и не смешиваются.

Theme может назначаться различным объектам, например Page или ComponentDefinition.

---

# 14. Binding

`Binding` связывает свойство ComponentInstance с источником данных.

Поддерживается простой путь:

```text
a.b.c
```

Источниками данных могут быть:

- Route parameters;
- Route query;
- props;
- Content;
- результат Operation;
- данные формы;
- другие runtime-источники.

Пример:

```text
ComponentInstance
    title ← article.title
```

Binding не содержит информации о конкретном протоколе получения данных.

---

# 15. Operation

`Operation` описывает действие над данными.

Operation делятся на:

| Тип      | Назначение                    |
| -------- | ----------------------------- |
| Query    | Получение данных              |
| Mutation | Изменение или отправка данных |

Operation является независимой сущностью.

Operation не зависит от других Operation.

Одна Operation может использоваться несколькими компонентами.

---

# 16. Provider

`Provider` определяет конкретный способ выполнения Operation.

Например:

- Local Database;
- HTTP;
- GraphQL;
- другие протоколы и системы.

Provider является расширяемой частью системы и может поставляться Plugin.

Компонент не знает, какой Provider используется.

---

# 17. Infrastructure

Infrastructure является уровнем, связывающим Operation с конкретными внешними или внутренними источниками данных.

Основная модель:

```text
Component
→ Binding
→ Operation
→ Provider
→ Data Source
```

Infrastructure может использовать:

- встроенную БД;
- HTTP API;
- GraphQL;
- другие внешние системы.

Сервис не реализует конкретные внешние протоколы как часть основного ядра. Их реализации поставляются расширениями.

Для внешних HTTP/GraphQL и подобных источников запрос выполняется непосредственно во frontend через Provider или Plugin. Backend хранит конфигурацию, версии и публичные параметры интеграции, но не проксирует такой запрос.

---

# 18. Endpoint

`Endpoint` представляет backend endpoint, необходимый frontend.

Endpoint может использовать Operation.

Endpoint не используется как прокси для внешних интеграций. Если источник доступен браузеру, frontend обращается к нему напрямую.

Пример:

```text
POST /contact
    → server-only Mutation
    → External CRM
```

Для публичной интеграции запрос выполняется frontend напрямую:

```text
Frontend
    → Mutation
    → HTTP Provider
    → External CRM
```

Также сервис может предоставлять существующие backend endpoints без преобразования.

Endpoint допускается только для операций, которые по своей природе должны выполняться на backend: работа с приватными секретами, локальной БД, webhooks или серверными административными действиями.

Сервис не должен превращаться в универсальный API Gateway.

---

# 19. Forms

Form является специальным ComponentDefinition.

Form содержит:

- поля;
- schema;
- validation;
- bindings;
- submit Operation.

Form может работать с локальной БД:

```text
Form
→ Mutation
→ Local Database
```

или с внешней системой:

```text
Form
→ Mutation
→ Frontend HTTP Provider
→ External System
```

Таким образом форма не зависит от конкретного backend-протокола.

---

# 20. Plugin

Plugin является механизмом расширения системы.

Plugin может предоставлять:

- внешние пакеты (fetch + lock из CDN/registry);
- React components;
- Component metadata;
- Themes;
- Providers;
- другие расширения frontend constructor.

Plugin не является отдельным backend gateway.

GraphQL, HTTP и другие внешние технологии не должны быть встроены непосредственно в основную модель сервиса, если их реализация может быть предоставлена Plugin.

---

# 21. Plugin Management

Система управляет frontend-зависимостями на уровне деклараций.

Сервер не имеет Node-окружения, поэтому к установке применяется fetch-модель:

1. получение описания пакета (`pkg@version`);
2. изменение package configuration;
3. загрузка предсобранного ESM-бандла с CDN/registry;
4. запись lockfile (+integrity) в версию и кэширование для пересборок;
5. включение Plugin в frontend build.

esbuild используется как build engine после подготовки frontend workspace.

---

# 22. Version

Каждый изменяемый тип объекта имеет собственное версионирование.

Например:

```text
ComponentVersion
PageVersion
RouteVersion
ThemeVersion
ContentVersion
OperationVersion
...
```

Изменение одного типа объекта не требует создания версии всего сайта.

Версии объектов являются неизменяемыми.

---

# 23. Snapshot

`Snapshot` представляет состояние сайта в определённый момент.

Snapshot содержит ссылки на конкретные версии объектов:

```text
Snapshot
    Components
    Pages
    Routes
    Themes
    Content
    Operations
    Infrastructure
    Plugins
```

Snapshot необходим для воспроизводимой сборки сайта.

Snapshot не заменяет версионирование отдельных объектов.

---

# 24. Build

`Build` представляет результат сборки конкретного Snapshot.

Build содержит:

- Site;
- Snapshot;
- Environment;
- build configuration;
- статус;
- результат сборки.

Процесс:

```text
Snapshot
→ Frontend Workspace
→ esbuild (внутри Go-бинарника)
→ static artifacts
```

Build должен быть привязан к конкретному Snapshot.

---

# 25. Environment

Environment определяет используемое состояние сайта.

Минимально:

```text
Development
Production
```

Environment указывает на используемый Snapshot/Build.

Development и Production не должны изменять друг друга.

Пример жизненного цикла:

```text
Object changes
→ new object versions
→ new Snapshot
→ Build
→ Development
→ testing
→ Production
```

Rollback выполняется выбором предыдущего Snapshot/Build.

---

# 26. Runtime

Runtime обрабатывает входящие frontend-запросы.

Основные функции:

- определение Site;
- определение Environment;
- обработка Route;
- выполнение Redirect;
- предоставление frontend;
- предоставление Assets;
- выполнение явно серверных Endpoints;
- выдача публичной конфигурации Operations;
- работа с cache.

Статическая frontend-сборка не должна требовать перестройки при каждом HTTP-запросе.

---

# 27. Build и Runtime

Build Time:

- собирает frontend;
- подключает компоненты;
- подключает темы;
- подключает plugins;
- разрешает внешние зависимости (fetch + lock из CDN/registry);
- запускает esbuild;
- создаёт static artifacts.

Runtime:

- отдаёт static artifacts;
- обрабатывает Routes;
- предоставляет Assets;
- выполняет только явно серверные endpoints;
- выдаёт frontend конфигурацию прямых Operations;
- получает динамические данные frontend напрямую от внешних систем;
- применяет cache policies.

---

# 28. Cache

Cache является отдельной настраиваемой политикой.

Кэширование может применяться к различным операциям и runtime-объектам.

Минимальные варианты:

| Policy    | Назначение                    |
| --------- | ----------------------------- |
| Disabled  | Без кэширования               |
| TTL       | Кэширование на заданное время |
| Immutable | Данные не меняются            |
| Custom    | Расширенная политика          |

Конкретная реализация cache может расширяться.

---

# 29. Site Isolation

Объекты могут использоваться несколькими сайтами, если это разрешено моделью.

При этом runtime должен однозначно определить:

```text
Site
→ Environment
→ Snapshot
→ Build
```

Сайт не должен зависеть от текущего состояния другого сайта.

Общие объекты могут переиспользоваться несколькими Site.

---

# 30. Основной жизненный цикл

### Создание сайта

```text
Create Site
→ Create Components
→ Create Content
→ Create Pages
→ Create Routes
→ Configure Themes
→ Configure Operations
→ Configure Plugins
```

### Изменение

```text
Edit Object
→ Create new Object Version
```

### Подготовка версии сайта

```text
Object Versions
→ Snapshot
```

### Сборка

```text
Snapshot
→ Build
→ esbuild
→ Static Artifacts
```

### Публикация

```text
Build
→ Production
```

---

# 31. Основной принцип архитектуры

Frontend-компонент должен описывать **только представление и ожидаемую структуру данных**.

Компонент не должен знать:

- где хранится Content;
- какой используется database;
- какой используется HTTP API;
- используется ли GraphQL;
- куда физически отправляется Mutation;
- как реализован cache.

Эти вопросы решаются через:

```text
Binding
Operation
Provider
Infrastructure
```

Таким образом один и тот же React-компонент может использовать локальные данные или данные внешней системы без изменения своего исходного кода.

## 32. Прямые интеграции frontend

Если внешний источник может безопасно принимать запрос из браузера, frontend обращается к нему напрямую. Go backend выступает control plane: публикует описание Operation, schema, публичную конфигурацию и версии, но не принимает и не пересылает payload внешней системы.

```text
Component
→ Binding
→ Operation
→ Frontend Provider / Plugin
→ External API
```

Серверный endpoint остаётся исключением для приватных credentials, локальной базы, server-only API, webhooks и действий, которым нельзя доверять браузеру. Всё, что отправлено браузеру, считается публичным.

Подробное правило описано в [docs/direct-integrations.md](docs/direct-integrations.md).

Актуальное состояние реализации и список доступных для проверки функций находятся в [docs/feature-status.md](docs/feature-status.md).

