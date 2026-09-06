// ../../../ui-runtime/src/errors.ts
var RuntimeError = class extends Error {
  constructor(code, message) {
    super(message);
    this.name = new.target.name;
    this.code = code;
  }
};
var DuplicateRegistrationError = class extends RuntimeError {
  constructor(message) {
    super("duplicate_registration", message);
  }
};
var UnknownEntityError = class extends RuntimeError {
  constructor(message) {
    super("unknown_entity", message);
  }
};
var UnknownProviderError = class extends RuntimeError {
  constructor(message) {
    super("unknown_provider", message);
  }
};
var OperationKindMismatchError = class extends RuntimeError {
  constructor(message) {
    super("operation_kind_mismatch", message);
  }
};
var RouteNotFoundError = class extends RuntimeError {
  constructor(message) {
    super("route_not_found", message);
  }
};
var TransportError = class extends RuntimeError {
  constructor(message, cause) {
    super("transport_error", message);
    this.cause = cause;
  }
};
var DescriptorValidationError = class extends RuntimeError {
  constructor(message, opts) {
    super("descriptor_validation", message);
    this.entityId = opts?.entityId;
    this.path = opts?.path;
  }
};
var SubscriptionError = class extends RuntimeError {
  constructor(message) {
    super("subscription_error", message);
  }
};
var ScopeError = class extends RuntimeError {
  constructor(message) {
    super("scope_error", message);
  }
};
var CronParseError = class extends RuntimeError {
  constructor(message) {
    super("cron_parse", message);
  }
};
var FormValidationError = class extends RuntimeError {
  constructor(message) {
    super("form_validation", message);
  }
};
var LocaleUnsupportedError = class extends RuntimeError {
  constructor(message) {
    super("locale_unsupported", message);
  }
};
var AssetNotFoundError = class extends RuntimeError {
  constructor(message) {
    super("asset_not_found", message);
  }
};

// ../../../ui-runtime/src/core/builtin/builtin-client.ts
var BuiltinClient = class {
  constructor(api) {
    this.api = api;
    this.content = {
      get: (contentId, opts) => this.api.query("content.get", { contentId, locale: opts?.locale }),
      list: (siteId, opts) => this.api.query("content.list", {
        siteId,
        collectionId: opts?.collectionId,
        locale: opts?.locale
      }),
      batch: (siteId, opts, localeOpts) => this.api.query("content.batch", { siteId, ids: opts.ids, locale: localeOpts?.locale })
    };
    this.asset = {
      get: async (assetId) => {
        try {
          return await this.api.query("asset.get", { assetId });
        } catch (e) {
          if (isNotFound(e)) throw new AssetNotFoundError(`\u0410\u0441\u0441\u0435\u0442 '${assetId}' \u043D\u0435 \u043D\u0430\u0439\u0434\u0435\u043D`);
          throw e;
        }
      },
      list: (siteId, _opts) => this.api.query("asset.list", { siteId }),
      batch: (ids, _localeOpts) => this.api.query("asset.batch", { ids })
    };
    this.form = {
      get: (formId) => this.api.query("form.get", { formId }),
      submit: (payload) => this.api.callEndpoint("form.submit", {
        formId: payload.formId,
        locale: payload.locale,
        submittedAt: payload.submittedAt,
        values: payload.values
      })
    };
  }
};
function isNotFound(e) {
  return e instanceof TransportError && e.cause?.status === 404;
}

// ../../../ui-runtime/src/core/cron.ts
var MONTH_NAMES = {
  JAN: 1,
  FEB: 2,
  MAR: 3,
  APR: 4,
  MAY: 5,
  JUN: 6,
  JUL: 7,
  AUG: 8,
  SEP: 9,
  OCT: 10,
  NOV: 11,
  DEC: 12
};
var DOW_NAMES = {
  SUN: 0,
  MON: 1,
  TUE: 2,
  WED: 3,
  THU: 4,
  FRI: 5,
  SAT: 6
};
function normalizeNames(value, names) {
  return value.split(",").map((part) => {
    const m = /^([A-Za-z]{3})/.exec(part);
    if (m && names[m[0].toUpperCase()] !== void 0) {
      const num = names[m[0].toUpperCase()];
      return part.replace(m[0], String(num));
    }
    return part;
  }).join(",");
}
function parseField(spec, min, max, names) {
  let field = spec.trim();
  if (field === "*") return void 0;
  if (names) field = normalizeNames(field, names);
  const out = /* @__PURE__ */ new Set();
  for (const part of field.split(",")) {
    let m = /^(\*|\d+)(?:-(\d+|\*))?(?:\/(\d+))?$/.exec(part);
    let step = 1;
    if (!m) {
      const slash = /^(\d+)-(\d+)\/(\d+)$/.exec(part);
      if (slash) {
        m = /^(\d+)-(\d+)(?:\/(\d+))?$/.exec(`${slash[1]}-${slash[2]}`);
        m = /^(\d+)-(\d+)(?:\/(\d+))?$/.exec(`${slash[1]}-${slash[2]}/${slash[3]}`);
        step = Number(slash[3]);
      }
    }
    if (!m) throw new CronParseError(`\u041D\u0435\u043A\u043E\u0440\u0440\u0435\u043A\u0442\u043D\u043E\u0435 \u043F\u043E\u043B\u0435 cron '${spec}'`);
    let start;
    let end;
    if (m[1] === "*") {
      start = min;
      end = m[2] && m[2] !== "*" ? Number(m[2]) : max;
      step = m[3] ? Number(m[3]) : 1;
    } else {
      start = Number(m[1]);
      end = m[2] ? Number(m[2]) : start;
      step = m[3] ? Number(m[3]) : 1;
    }
    if (start < min || end > max || start > end || step < 1) {
      throw new CronParseError(`\u0417\u043D\u0430\u0447\u0435\u043D\u0438\u0435 \u0432\u043D\u0435 \u0434\u0438\u0430\u043F\u0430\u0437\u043E\u043D\u0430 [${min}..${max}] \u0432 '${spec}'`);
    }
    for (let v = start; v <= end; v += step) out.add(v);
  }
  return Array.from(out).sort((a, b) => a - b);
}
function parseCron(expr) {
  if (typeof expr !== "string" || expr.trim().length === 0) {
    throw new CronParseError("Cron-\u0432\u044B\u0440\u0430\u0436\u0435\u043D\u0438\u0435 \u043D\u0435 \u0437\u0430\u0434\u0430\u043D\u043E");
  }
  const parts = expr.trim().split(/\s+/);
  if (parts.length !== 6) {
    throw new CronParseError(`Cron \u0434\u043E\u043B\u0436\u0435\u043D \u0441\u043E\u0434\u0435\u0440\u0436\u0430\u0442\u044C 6 \u043F\u043E\u043B\u0435\u0439 (sec min hour dom mon dow), \u043F\u043E\u043B\u0443\u0447\u0435\u043D\u043E ${parts.length}: '${expr}'`);
  }
  try {
    return {
      sec: parseField(parts[0], 0, 59),
      min: parseField(parts[1], 0, 59),
      hour: parseField(parts[2], 0, 23),
      dom: parseField(parts[3], 1, 31),
      mon: parseField(parts[4], 1, 12, MONTH_NAMES),
      dow: parseField(parts[5], 0, 6, DOW_NAMES),
      raw: expr.trim()
    };
  } catch (err) {
    if (err instanceof CronParseError) throw err;
    throw new CronParseError(`\u041D\u0435 \u0443\u0434\u0430\u043B\u043E\u0441\u044C \u0440\u0430\u0437\u043E\u0431\u0440\u0430\u0442\u044C cron '${expr}': ${err instanceof Error ? err.message : String(err)}`);
  }
}
function matchesMonth(d, mon) {
  if (!mon) return true;
  return mon.includes(d.getMonth() + 1);
}
function matchesDowDom(d, dom, dow) {
  if (dom === void 0 && dow === void 0) return true;
  if (dom !== void 0 && dow !== void 0) {
    return dom.includes(d.getDate()) || dow.includes(d.getDay());
  }
  if (dom !== void 0) return dom.includes(d.getDate());
  return dow.includes(d.getDay());
}
function nextTick(now, expr) {
  const d = new Date(now.getTime() + 1e3);
  d.setMilliseconds(0);
  const guard = 31536e3;
  for (let i = 0; i < guard; i++) {
    if (!matchesMonth(d, expr.mon)) {
      advanceDay(d);
      continue;
    }
    if (!matchesDowDom(d, expr.dom, expr.dow)) {
      advanceDay(d);
      continue;
    }
    if (expr.hour && !expr.hour.includes(d.getHours())) {
      advanceHour(d);
      continue;
    }
    if (expr.min && !expr.min.includes(d.getMinutes())) {
      advanceMinute(d);
      continue;
    }
    if (expr.sec && !expr.sec.includes(d.getSeconds())) {
      d.setSeconds(d.getSeconds() + 1);
      continue;
    }
    return d;
  }
  throw new CronParseError("\u041D\u0435 \u0443\u0434\u0430\u043B\u043E\u0441\u044C \u043D\u0430\u0439\u0442\u0438 \u0441\u043B\u0435\u0434\u0443\u044E\u0449\u0443\u044E \u0442\u043E\u0447\u043A\u0443 \u0432\u044B\u043F\u043E\u043B\u043D\u0435\u043D\u0438\u044F cron");
}
function advanceDay(d) {
  d.setDate(d.getDate() + 1);
  d.setHours(0, 0, 0, 0);
}
function advanceHour(d) {
  d.setHours(d.getHours() + 1, 0, 0, 0);
}
function advanceMinute(d) {
  d.setMinutes(d.getMinutes() + 1, 0, 0);
}

// ../../../ui-runtime/src/core/poll-scheduler.ts
var PollScheduler = class {
  constructor(env) {
    this.jobs = /* @__PURE__ */ new Map();
    this.counter = 0;
    this.impl = {
      setTimeout: env?.setTimeout ?? ((fn, ms) => setTimeout(fn, ms)),
      clearTimeout: env?.clearTimeout ?? ((t) => clearTimeout(t)),
      now: env?.now ?? (() => /* @__PURE__ */ new Date())
    };
  }
  add(schedule, tick, opts) {
    let cronExpr;
    if (typeof schedule === "string") {
      cronExpr = parseCron(schedule);
    } else if (typeof schedule !== "number") {
      throw new CronParseError("\u0420\u0430\u0441\u043F\u0438\u0441\u0430\u043D\u0438\u0435 \u0434\u043E\u043B\u0436\u043D\u043E \u0431\u044B\u0442\u044C cron-\u0441\u0442\u0440\u043E\u043A\u043E\u0439 \u0438\u043B\u0438 \u0447\u0438\u0441\u043B\u043E\u043C (\u043C\u0441)");
    }
    const id = opts?.id ?? `job-${++this.counter}`;
    const delay = () => {
      if (cronExpr) {
        return Math.max(1, nextTick(this.impl.now(), cronExpr).getTime() - this.impl.now().getTime());
      }
      return Math.max(1, Math.trunc(schedule));
    };
    const internal = {
      id,
      delay,
      tick,
      errorHandler: opts?.errorHandler,
      running: false,
      dead: false,
      cancel: () => this.remove(id)
    };
    this.jobs.set(id, internal);
    const arm = () => {
      if (internal.dead) return;
      const ms = internal.delay();
      let token;
      token = this.impl.setTimeout(() => this.fire(id, internal, token), ms);
      internal.token = token;
    };
    if (opts?.immediate === false) {
      arm();
    } else {
      void this.run(internal).then(() => {
        if (!internal.dead) arm();
      });
    }
    return internal;
  }
  run(job) {
    job.running = true;
    let p;
    try {
      p = Promise.resolve(job.tick());
    } catch (e) {
      p = Promise.reject(e);
    }
    return p.catch((e) => {
      if (job.errorHandler) job.errorHandler(e);
    }).then(() => {
      job.running = false;
    });
  }
  fire(id, job, token) {
    if (job.dead || job.token !== token) return;
    job.token = void 0;
    if (job.running) return;
    void this.run(job).then(() => {
      if (!job.dead && !job.running) {
        const ms = job.delay();
        let t2;
        t2 = this.impl.setTimeout(() => this.fire(id, job, t2), ms);
        job.token = t2;
      }
    });
  }
  remove(id) {
    const job = this.jobs.get(id);
    if (!job) return;
    job.dead = true;
    if (job.token) this.impl.clearTimeout(job.token);
    this.jobs.delete(id);
  }
  size() {
    return this.jobs.size;
  }
  has(id) {
    const job = this.jobs.get(id);
    return !!job && !job.dead;
  }
  clear() {
    for (const id of Array.from(this.jobs.keys())) this.remove(id);
  }
};

// ../../../ui-runtime/src/core/sync.ts
var SyncEngine = class {
  constructor(registry, transport, opts) {
    this.registry = registry;
    this.transport = transport;
    this.counter = 0;
    this.scheduler = new PollScheduler(opts?.schedulerEnv);
  }
  /** Short polling по operationId: каждый тик — отдельный HTTP-запрос. Возвращает отписку. */
  poll(operationId, onData, opts) {
    const op = this.requireOperation(operationId);
    const schedule = op.poll?.schedule ?? opts?.schedule;
    if (!schedule) {
      throw new DescriptorValidationError(`\u041E\u043F\u0435\u0440\u0430\u0446\u0438\u044F '${op.id}' \u043D\u0435 \u0438\u043C\u0435\u0435\u0442 poll.schedule \u0438 \u043D\u0435 \u043F\u0435\u0440\u0435\u0434\u0430\u043D opts.schedule`);
    }
    const meta = { timestamp: Date.now(), channel: "poll" };
    const self = this;
    const id = `poll-${++self.counter}`;
    const tick = () => self.executePoll(op, opts, (data) => onData(data, { ...meta, timestamp: Date.now() }));
    const job = this.scheduler.add(schedule, tick, {
      immediate: opts?.immediate ?? true,
      errorHandler: opts?.errorHandler,
      id
    });
    return () => job.cancel();
  }
  /** Подписка по каналу: opts.channel → иначе push (ws/sse), если есть; иначе poll. */
  subscribe(operationId, onData, opts) {
    const op = this.requireOperation(operationId);
    const channel = this.pickChannel(op, opts?.channel);
    const meta = { timestamp: Date.now(), channel };
    if (channel === "poll") {
      return this.poll(operationId, onData, opts);
    }
    if (typeof this.transport.subscribe !== "function") {
      throw new DescriptorValidationError(`\u0422\u0440\u0430\u043D\u0441\u043F\u043E\u0440\u0442 \u043F\u0440\u043E\u0432\u0430\u0439\u0434\u0435\u0440\u0430 '${op.provider.id}' \u043D\u0435 \u043F\u043E\u0434\u0434\u0435\u0440\u0436\u0438\u0432\u0430\u0435\u0442 push-\u043A\u0430\u043D\u0430\u043B ${channel}`);
    }
    const req = { provider: op.provider, operation: op, input: opts?.input, params: opts?.params };
    return this.transport.subscribe(
      req,
      (data) => onData(data, { ...meta, timestamp: Date.now() }),
      (e) => {
        if (opts?.errorHandler) opts.errorHandler(e);
      }
    );
  }
  requireOperation(operationId) {
    const op = this.registry.findOperationByIdOrBadge(operationId) ?? this.registry.getOperation(operationId);
    if (!op) throw new DescriptorValidationError(`\u041D\u0435\u0438\u0437\u0432\u0435\u0441\u0442\u043D\u0430\u044F \u043E\u043F\u0435\u0440\u0430\u0446\u0438\u044F '${operationId}'`);
    return op;
  }
  pickChannel(op, forced) {
    if (forced === "poll") return "poll";
    if (forced === "sse" || forced === "ws") {
      if (!op.subscribe) throw new DescriptorValidationError(`\u041E\u043F\u0435\u0440\u0430\u0446\u0438\u044F '${op.id}' \u043D\u0435 \u043C\u043E\u0436\u0435\u0442 \u0440\u0430\u0431\u043E\u0442\u0430\u0442\u044C \u043F\u043E \u043A\u0430\u043D\u0430\u043B\u0443 ${forced}: \u043D\u0435\u0442 subscribe`);
      return forced;
    }
    const isPush = op.push === true || op.provider.subscribe !== void 0;
    if (isPush && op.subscribe) {
      return op.provider.protocol === "sse" ? "sse" : "ws";
    }
    return "poll";
  }
  async executePoll(op, opts, emit) {
    const res = await this.transport.request({
      provider: op.provider,
      operation: op,
      input: opts?.input,
      params: opts?.params
    });
    emit(res.body);
  }
  get activeSize() {
    return this.scheduler.size();
  }
  /** Отменяет все poll-задания (при перезагрузке runtime). */
  dispose() {
    this.scheduler.clear();
  }
};

// ../../../ui-runtime/src/core/api-client.ts
var ApiClient = class {
  constructor(registry, opts) {
    this.registry = registry;
    this.opts = opts;
    this.cache = /* @__PURE__ */ new Map();
    this.scope = opts.scope ?? "public";
    this.cacheEnabled = opts.cache ?? true;
    this.sync = opts.sync ?? new SyncEngine(registry, opts.transport);
    this.builtin = new BuiltinClient({
      query: (operationId, input) => this.query(operationId, input),
      callEndpoint: (endpointId, input) => this.callEndpoint(endpointId, input)
    });
  }
  async query(operationId, input) {
    const op = this.resolveOperation("query", operationId);
    return await this.execute(op, input);
  }
  async mutate(operationId, input) {
    const op = this.resolveOperation("mutation", operationId);
    return await this.execute(op, input);
  }
  /** Server-only: вызов серверной операции по эндпоинту. */
  async callEndpoint(endpointId, input) {
    if (this.scope !== "server") {
      throw new ScopeError(`Endpoint '${endpointId}' \u0434\u043E\u0441\u0442\u0443\u043F\u0435\u043D \u0442\u043E\u043B\u044C\u043A\u043E \u0438\u0437 server-\u043A\u043E\u043D\u0442\u0435\u043A\u0441\u0442\u0430`);
    }
    const endpoint = this.registry.getEndpoint(endpointId);
    if (!endpoint) {
      throw new UnknownEntityError(`Endpoint '${endpointId}' \u043D\u0435 \u043D\u0430\u0439\u0434\u0435\u043D`);
    }
    return await this.execute(endpoint.operation, input);
  }
  /** Push-подписка (ws/sse); rollback к poll, если push не объявлен. Возвращает отписку. */
  subscribe(operationId, onData) {
    return this.sync.subscribe(operationId, onData);
  }
  /** Short polling по operationId; opts.schedule = cron, параметры — из input. */
  poll(operationId, onData, opts) {
    return this.sync.poll(operationId, onData, { ...opts, params: opts?.input });
  }
  resolveOperation(typeOp, id) {
    const direct = this.registry.findOperationByIdOrBadge(id) ?? this.registry.getOperation(id);
    if (direct) {
      if (direct.typeOp !== typeOp) {
        throw new OperationKindMismatchError(`\u041E\u043F\u0435\u0440\u0430\u0446\u0438\u044F '${direct.id}' \u2014 '${direct.typeOp}', \u0430 \u0442\u0440\u0435\u0431\u0443\u0435\u0442\u0441\u044F '${typeOp}'`);
      }
      return direct;
    }
    const op = this.registry.getOperationFor(typeOp, id);
    if (op) return op;
    throw new UnknownEntityError(`\u041E\u043F\u0435\u0440\u0430\u0446\u0438\u044F '${id}' \u043D\u0435 \u043D\u0430\u0439\u0434\u0435\u043D\u0430`);
  }
  async execute(op, input) {
    const policy = op.cache ?? "disabled";
    const key = this.cacheEnabled && policy !== "disabled" ? cacheKey(op.id, input) : void 0;
    if (key !== void 0) {
      const hit = this.cache.get(key);
      if (hit) {
        if (hit.expiresAt === void 0 || hit.expiresAt > Date.now()) return hit.value;
        this.cache.delete(key);
      }
    }
    const req = {
      provider: op.provider,
      operation: op,
      params: { ...input ?? {} }
    };
    const res = await this.opts.transport.request(req);
    if (key !== void 0) {
      this.cache.set(key, {
        value: res.body,
        expiresAt: policy === "ttl" && typeof op.ttl === "number" ? Date.now() + op.ttl * 1e3 : void 0
      });
    }
    return res.body;
  }
};
function cacheKey(operationId, input) {
  return `${operationId}\0${JSON.stringify(input ?? {})}`;
}

// ../../../ui-runtime/src/core/assets.ts
var AssetResolver = class {
  constructor(store, assets) {
    this.store = store;
    this.assets = assets;
  }
  /** Метаданные ассета; второй вызов — из кэша `store.assets` (без запроса). */
  async get(assetId, opts) {
    const cached = this.store.getState().assets[assetId];
    if (cached && !opts?.force) return cached;
    const meta = await this.assets.get(assetId);
    this.store.getState().setAssets({ ...this.store.getState().assets, [assetId]: meta });
    return meta;
  }
  /** URL варианта; default 'master'. Неизвестный assetId/вариант → undefined (не бросает). */
  url(ref) {
    const meta = this.store.getState().assets[ref.assetId];
    if (!meta) return void 0;
    const variant = ref.variant ?? "master";
    const v = meta.variants.find((x) => x.name === variant);
    return v?.url;
  }
  /** Глубокий обход: объекты вида `{ assetId, variant? }` → строка URL; иначе значение остаётся как есть. */
  resolveDeep(data) {
    return walk(this.url.bind(this), data);
  }
};
function walk(urlOf, value) {
  if (Array.isArray(value)) return value.map((v) => walk(urlOf, v));
  if (value !== null && typeof value === "object") {
    const record = value;
    if (typeof record.assetId === "string") {
      const resolved = urlOf({ assetId: record.assetId, variant: typeof record.variant === "string" ? record.variant : void 0 });
      return resolved ?? value;
    }
    const out = {};
    for (const [k, v] of Object.entries(record)) out[k] = walk(urlOf, v);
    return out;
  }
  return value;
}

// ../../../ui-runtime/src/core/builtin/descriptors.ts
var BUILTIN_PROVIDER_ID = "liapoldus.builtin";
var builtinProviderDescriptor = {
  kind: "provider",
  id: BUILTIN_PROVIDER_ID,
  protocol: "http"
};
var builtinOperationDescriptors = [
  {
    kind: "operation",
    id: "content.get",
    typeOp: "query",
    providerId: BUILTIN_PROVIDER_ID,
    method: "GET",
    path: "/api/contents/{contentId}",
    params: { in: "query", fields: { contentId: { required: true }, locale: { required: false } } },
    cache: "immutable",
    type: "content"
  },
  {
    kind: "operation",
    id: "content.list",
    typeOp: "query",
    providerId: BUILTIN_PROVIDER_ID,
    method: "GET",
    path: "/api/sites/{siteId}/contents",
    params: {
      in: "query",
      fields: { siteId: { required: true }, collectionId: { required: false }, locale: { required: false } }
    },
    cache: "ttl",
    ttl: 120,
    type: "content[]"
  },
  {
    kind: "operation",
    id: "content.batch",
    typeOp: "query",
    providerId: BUILTIN_PROVIDER_ID,
    method: "POST",
    path: "/api/sites/{siteId}/contents/batch",
    params: { in: "body" },
    cache: "immutable",
    type: "content[]"
  },
  {
    kind: "operation",
    id: "asset.get",
    typeOp: "query",
    providerId: BUILTIN_PROVIDER_ID,
    method: "GET",
    path: "/api/assets/{assetId}",
    params: { in: "query", fields: { assetId: { required: true } } },
    cache: "immutable",
    type: "asset"
  },
  {
    kind: "operation",
    id: "asset.list",
    typeOp: "query",
    providerId: BUILTIN_PROVIDER_ID,
    method: "GET",
    path: "/api/sites/{siteId}/assets",
    params: { in: "query", fields: { siteId: { required: true } } },
    cache: "immutable",
    type: "asset[]"
  },
  {
    kind: "operation",
    id: "asset.batch",
    typeOp: "query",
    providerId: BUILTIN_PROVIDER_ID,
    method: "POST",
    path: "/api/sites/{siteId}/assets/batch",
    params: { in: "body" },
    cache: "immutable",
    type: "assets"
  },
  {
    kind: "operation",
    id: "form.get",
    typeOp: "query",
    providerId: BUILTIN_PROVIDER_ID,
    method: "GET",
    path: "/api/forms/{formId}",
    params: { in: "query", fields: { formId: { required: true } } },
    cache: "ttl",
    ttl: 300,
    type: "form"
  },
  {
    kind: "operation",
    id: "form.submit",
    typeOp: "mutation",
    providerId: BUILTIN_PROVIDER_ID,
    method: "POST",
    path: "/api/forms/{formId}/submissions",
    params: { in: "path", fields: { formId: { required: true } } },
    cache: "disabled",
    scope: "server"
  },
  {
    kind: "operation",
    id: "tree.get",
    typeOp: "query",
    providerId: BUILTIN_PROVIDER_ID,
    method: "GET",
    path: "/runtime/tree",
    params: { in: "query", fields: { routeId: { required: false }, locale: { required: false } } },
    cache: "immutable",
    type: "tree"
  },
  {
    kind: "operation",
    id: "tokens.get",
    typeOp: "query",
    providerId: BUILTIN_PROVIDER_ID,
    method: "GET",
    path: "/runtime/tokens",
    params: { in: "query", fields: { themeId: { required: false } } },
    cache: "immutable",
    type: "tokens"
  },
  {
    kind: "operation",
    id: "routes.get",
    typeOp: "query",
    providerId: BUILTIN_PROVIDER_ID,
    method: "GET",
    path: "/runtime/routes",
    params: { in: "query" },
    cache: "immutable",
    type: "routes[]"
  }
];
var builtinEndpointDescriptors = [
  {
    kind: "endpoint",
    id: "form.submit",
    path: "/api/forms/{formId}/submissions",
    method: "POST",
    operationId: "form.submit"
  }
];
function registerBuiltin(registry, opts) {
  const providerId = opts?.providerId ?? BUILTIN_PROVIDER_ID;
  if (!registry.hasProvider(providerId)) {
    registry.registerProvider({ ...builtinProviderDescriptor, id: providerId });
  }
  const ops = opts?.providerId ? builtinOperationDescriptors.map((op) => op.providerId === BUILTIN_PROVIDER_ID ? { ...op, providerId: opts.providerId } : op) : builtinOperationDescriptors;
  for (const op of ops) {
    if (!registry.hasOperation(op.id)) registry.registerOperation(op);
  }
  for (const ep of builtinEndpointDescriptors) {
    if (!registry.hasEndpoint(ep.id)) registry.registerEndpoint(ep);
  }
}

// ../../../ui-runtime/src/core/descriptor.ts
var BADGE_RE = /^([A-Za-z0-9._/-]+)#([a-z]+)$/;
function stripComments(json) {
  return json.replace(/\/\*[\s\S]*?\*\//g, "").replace(/(^|[^\\:])\/\/[^\n\r]*/g, "$1").replace(/,(\s*[}\]])/g, "$1");
}
function parseContractJSON(json) {
  try {
    return JSON.parse(stripComments(json));
  } catch (err) {
    throw new DescriptorValidationError(
      `\u041E\u0448\u0438\u0431\u043A\u0430 JSON: ${err instanceof Error ? err.message : String(err)}`
    );
  }
}
function isRecord(v) {
  return typeof v === "object" && v !== null && !Array.isArray(v);
}
function requireString(obj, field, entityId) {
  const v = obj[field];
  if (typeof v !== "string" || v.length === 0) {
    throw new DescriptorValidationError(`\u041F\u043E\u043B\u0435 '${field}' \u0434\u043E\u043B\u0436\u043D\u043E \u0431\u044B\u0442\u044C \u043D\u0435\u043F\u0443\u0441\u0442\u043E\u0439 \u0441\u0442\u0440\u043E\u043A\u043E\u0439`, { entityId, path: field });
  }
  return v;
}
function splitBadgedId(id) {
  const m = BADGE_RE.exec(id);
  if (m) return { name: m[1], badge: m[2] };
  return { name: id };
}
function validateProviderDescriptor(raw) {
  if (!isRecord(raw)) throw new DescriptorValidationError("\u0414\u0435\u0441\u043A\u0440\u0438\u043F\u0442\u043E\u0440 provider \u0434\u043E\u043B\u0436\u0435\u043D \u0431\u044B\u0442\u044C \u043E\u0431\u044A\u0435\u043A\u0442\u043E\u043C");
  const id = requireString(raw, "id", "provider");
  const protocol = raw.protocol;
  if (protocol !== "http" && protocol !== "ws" && protocol !== "sse" && protocol !== "graphql") {
    throw new DescriptorValidationError(`\u041F\u0440\u043E\u0442\u043E\u043A\u043E\u043B '${String(protocol)}' \u043D\u0435 \u043F\u043E\u0434\u0434\u0435\u0440\u0436\u0438\u0432\u0430\u0435\u0442\u0441\u044F`, { entityId: id, path: "protocol" });
  }
  const baseUrlValue = raw.baseUrl;
  if (baseUrlValue !== void 0 && typeof baseUrlValue !== "string") {
    throw new DescriptorValidationError("baseUrl \u0434\u043E\u043B\u0436\u0435\u043D \u0431\u044B\u0442\u044C \u0441\u0442\u0440\u043E\u043A\u043E\u0439", { entityId: id, path: "baseUrl" });
  }
  const provider = {
    kind: "provider",
    id,
    protocol
  };
  if (typeof baseUrlValue === "string") provider.baseUrl = baseUrlValue;
  if (isRecord(raw.defaults)) {
    const defaults = {};
    if (isRecord(raw.defaults.headers)) defaults.headers = raw.defaults.headers;
    if (typeof raw.defaults.timeoutMs === "number") defaults.timeoutMs = raw.defaults.timeoutMs;
    if (Object.keys(defaults).length > 0) provider.defaults = defaults;
  }
  if (isRecord(raw.subscribe)) {
    const subscribe = {};
    if (typeof raw.subscribe.url === "string") subscribe.url = raw.subscribe.url;
    if (typeof raw.subscribe.eventName === "string") subscribe.eventName = raw.subscribe.eventName;
    provider.subscribe = subscribe;
  }
  return provider;
}
var METHODS = ["GET", "POST", "PUT", "PATCH", "DELETE"];
var OP_TYPES = ["query", "mutation"];
var SCOPES = ["public", "server"];
var TYPED_BINDINGS = ["content", "content[]", "asset", "asset[]", "assets", "form", "tree", "tokens", "routes[]"];
function validateOperationDescriptor(raw) {
  if (!isRecord(raw)) throw new DescriptorValidationError("\u0414\u0435\u0441\u043A\u0440\u0438\u043F\u0442\u043E\u0440 operation \u0434\u043E\u043B\u0436\u0435\u043D \u0431\u044B\u0442\u044C \u043E\u0431\u044A\u0435\u043A\u0442\u043E\u043C");
  const id = requireString(raw, "id", "operation");
  const typeOp = raw.typeOp ?? raw.type_operation;
  if (!OP_TYPES.includes(typeOp)) {
    throw new DescriptorValidationError(`type \u0434\u043E\u043B\u0436\u0435\u043D \u0431\u044B\u0442\u044C 'query' \u0438\u043B\u0438 'mutation'`, { entityId: id, path: "typeOp" });
  }
  requireString(raw, "providerId", id);
  const method = raw.method ?? "GET";
  if (!METHODS.includes(method)) {
    throw new DescriptorValidationError(`method '${String(method)}' \u043D\u0435 \u043F\u043E\u0434\u0434\u0435\u0440\u0436\u0438\u0432\u0430\u0435\u0442\u0441\u044F`, { entityId: id, path: "method" });
  }
  const cache = raw.cache;
  if (cache !== "immutable" && cache !== "disabled" && cache !== "ttl") {
    throw new DescriptorValidationError("cache \u0434\u043E\u043B\u0436\u0435\u043D \u0431\u044B\u0442\u044C immutable|disabled|ttl", { entityId: id, path: "cache" });
  }
  const scope = raw.scope ?? "public";
  if (!SCOPES.includes(scope)) {
    throw new DescriptorValidationError("scope \u0434\u043E\u043B\u0436\u0435\u043D \u0431\u044B\u0442\u044C public|server", { entityId: id, path: "scope" });
  }
  const op = {
    kind: "operation",
    id,
    typeOp,
    providerId: raw.providerId,
    method,
    cache,
    scope
  };
  if (typeof raw.path === "string") op.path = raw.path;
  if (typeof raw.ttl === "number") op.ttl = raw.ttl;
  if (raw.push === true) op.push = true;
  if (isRecord(raw.params)) {
    const pin = raw.params.in;
    if (pin !== "query" && pin !== "path" && pin !== "body") {
      throw new DescriptorValidationError("params.in \u0434\u043E\u043B\u0436\u0435\u043D \u0431\u044B\u0442\u044C query|path|body", { entityId: id, path: "params.in" });
    }
    const params = { in: pin };
    if (isRecord(raw.params.fields)) {
      params.fields = {};
      for (const [field, spec] of Object.entries(raw.params.fields)) {
        params.fields[field] = { required: isRecord(spec) && spec.required === true };
      }
    }
    op.params = params;
  }
  if (isRecord(raw.input) && typeof raw.input.schemaId === "string") op.input = { schemaId: raw.input.schemaId };
  if (isRecord(raw.output) && typeof raw.output.schemaId === "string") op.output = { schemaId: raw.output.schemaId };
  if (typeof raw.type === "string") {
    if (!TYPED_BINDINGS.includes(raw.type) && raw.type !== "string" && raw.type !== "raw") {
      throw new DescriptorValidationError(`type '${raw.type}' \u043D\u0435 \u044F\u0432\u043B\u044F\u0435\u0442\u0441\u044F \u0438\u0437\u0432\u0435\u0441\u0442\u043D\u043E\u0439 \u043F\u0440\u0438\u0432\u044F\u0437\u043A\u043E\u0439`, { entityId: id, path: "type" });
    }
    op.type = raw.type;
  }
  if (typeof raw.poll === "object" && raw.poll !== null) {
    const poll = raw.poll;
    if (typeof poll.schedule !== "string" && typeof poll.intervalMs !== "number") {
      throw new DescriptorValidationError("poll \u0442\u0440\u0435\u0431\u0443\u0435\u0442 schedule (cron) \u0438\u043B\u0438 intervalMs", { entityId: id, path: "poll" });
    }
    op.poll = { schedule: typeof poll.schedule === "string" ? poll.schedule : "" };
    if (typeof poll.intervalMs === "number") op.poll.intervalMs = poll.intervalMs;
    if (typeof poll.cache === "boolean") op.poll.cache = poll.cache;
  }
  if (isRecord(raw.subscribe)) {
    const subscribe = {};
    if (typeof raw.subscribe.url === "string") subscribe.url = raw.subscribe.url;
    if (typeof raw.subscribe.eventName === "string") subscribe.eventName = raw.subscribe.eventName;
    op.subscribe = subscribe;
  }
  if (isRecord(raw.headers)) op.headers = raw.headers;
  return op;
}
function validateEndpointDescriptor(raw) {
  if (!isRecord(raw)) throw new DescriptorValidationError("\u0414\u0435\u0441\u043A\u0440\u0438\u043F\u0442\u043E\u0440 endpoint \u0434\u043E\u043B\u0436\u0435\u043D \u0431\u044B\u0442\u044C \u043E\u0431\u044A\u0435\u043A\u0442\u043E\u043C");
  const id = requireString(raw, "id", "endpoint");
  requireString(raw, "operationId", id);
  const method = raw.method ?? "POST";
  if (!METHODS.includes(method)) {
    throw new DescriptorValidationError(`method '${String(method)}' \u043D\u0435 \u043F\u043E\u0434\u0434\u0435\u0440\u0436\u0438\u0432\u0430\u0435\u0442\u0441\u044F`, { entityId: id, path: "method" });
  }
  const ep = {
    kind: "endpoint",
    id,
    path: typeof raw.path === "string" ? raw.path : "/",
    method,
    operationId: raw.operationId
  };
  if (isRecord(raw.input) && typeof raw.input.schemaId === "string") ep.input = { schemaId: raw.input.schemaId };
  if (isRecord(raw.output) && typeof raw.output.schemaId === "string") ep.output = { schemaId: raw.output.schemaId };
  return ep;
}
function validateRouteDescriptor(raw) {
  if (!isRecord(raw)) throw new DescriptorValidationError("\u0414\u0435\u0441\u043A\u0440\u0438\u043F\u0442\u043E\u0440 route \u0434\u043E\u043B\u0436\u0435\u043D \u0431\u044B\u0442\u044C \u043E\u0431\u044A\u0435\u043A\u0442\u043E\u043C");
  const id = requireString(raw, "id", "route");
  const matcher = requireString(raw, "matcher", id);
  if (!matcher.startsWith("^") || !matcher.endsWith("$")) {
    throw new DescriptorValidationError(
      `matcher \u043C\u0430\u0440\u0448\u0440\u0443\u0442\u0430 '${id}' \u0434\u043E\u043B\u0436\u0435\u043D \u0431\u044B\u0442\u044C \u043F\u043E\u043B\u043D\u044B\u043C regex \u0441 \u044F\u043A\u043E\u0440\u044F\u043C\u0438 ^\u2026$`,
      { entityId: id, path: "matcher" }
    );
  }
  new RegExp(matcher);
  const priority = typeof raw.priority === "number" ? raw.priority : 0;
  if (!isRecord(raw.action)) {
    throw new DescriptorValidationError("route \u0442\u0440\u0435\u0431\u0443\u0435\u0442 action", { entityId: id, path: "action" });
  }
  const type = raw.action.type;
  if (type === "renderPage") {
    requireString(raw.action, "pageId", id);
  } else if (type === "serveAsset") {
    requireString(raw.action, "assetId", id);
  } else if (type === "redirect") {
    requireString(raw.action, "target", id);
    const status = raw.action.status;
    if (status !== void 0 && (status !== 301 && status !== 302 && status !== 307 && status !== 308)) {
      throw new DescriptorValidationError(
        `status \u0440\u0435\u0434\u0438\u0440\u0435\u043A\u0442\u0430 \u043C\u0430\u0440\u0448\u0440\u0443\u0442\u0430 '${id}' \u0432\u043D\u0435 {301,302,307,308}`,
        { entityId: id, path: "action.status" }
      );
    }
  } else {
    throw new DescriptorValidationError(`action.type \u043C\u0430\u0440\u0448\u0440\u0443\u0442\u0430 '${id}' \u0434\u043E\u043B\u0436\u0435\u043D \u0431\u044B\u0442\u044C renderPage|serveAsset|redirect`, {
      entityId: id,
      path: "action.type"
    });
  }
  return { kind: "route", id, matcher, priority, action: raw.action };
}
function validateThemeDescriptor(raw) {
  if (!isRecord(raw)) throw new DescriptorValidationError("\u0414\u0435\u0441\u043A\u0440\u0438\u043F\u0442\u043E\u0440 theme \u0434\u043E\u043B\u0436\u0435\u043D \u0431\u044B\u0442\u044C \u043E\u0431\u044A\u0435\u043A\u0442\u043E\u043C");
  const themeId = requireString(raw, "themeId", "theme");
  const tokens = raw.tokens;
  if (!isRecord(tokens)) {
    throw new DescriptorValidationError("theme \u0442\u0440\u0435\u0431\u0443\u0435\u0442 tokens", { entityId: themeId, path: "tokens" });
  }
  for (const [name, def] of Object.entries(tokens)) {
    if (typeof def === "string") continue;
    if (isRecord(def) && typeof def.value === "string") continue;
    if (isRecord(def) && typeof def.ref === "string") continue;
    throw new DescriptorValidationError(
      `\u0422\u043E\u043A\u0435\u043D '${name}' \u0442\u0435\u043C\u044B '${themeId}' \u0434\u043E\u043B\u0436\u0435\u043D \u0431\u044B\u0442\u044C string | { value } | { ref }`,
      { entityId: themeId, path: `tokens.${name}` }
    );
  }
  const theme = {
    kind: "theme",
    themeId,
    tokens: { ...tokens }
  };
  if (Array.isArray(raw.fonts)) theme.fonts = raw.fonts.filter((f) => typeof f === "string");
  if (Array.isArray(raw.assets)) theme.assets = raw.assets.filter((a) => typeof a === "string");
  return theme;
}
function parseDescriptors(json) {
  const raw = parseContractJSON(json);
  if (!isRecord(raw)) throw new DescriptorValidationError("\u041A\u043E\u0440\u0435\u043D\u044C \u043A\u043E\u043D\u0442\u0440\u0430\u043A\u0442\u0430 \u0434\u043E\u043B\u0436\u0435\u043D \u0431\u044B\u0442\u044C \u043E\u0431\u044A\u0435\u043A\u0442\u043E\u043C");
  const list = (key) => {
    const arr = raw[key];
    if (arr === void 0) return [];
    if (!Array.isArray(arr)) throw new DescriptorValidationError(`'${key}' \u0434\u043E\u043B\u0436\u0435\u043D \u0431\u044B\u0442\u044C \u043C\u0430\u0441\u0441\u0438\u0432\u043E\u043C`, { path: key });
    return arr;
  };
  const providers = list("providers").map(validateProviderDescriptor);
  const operations = list("operations").map(validateOperationDescriptor);
  const endpoints = list("endpoints").map(validateEndpointDescriptor);
  const routes = list("routes").map(validateRouteDescriptor);
  const themes = list("themes").map(validateThemeDescriptor);
  const enabledChannels = { ws: true, sse: true };
  if (isRecord(raw.enabledChannels)) {
    if (typeof raw.enabledChannels.ws === "boolean") enabledChannels.ws = raw.enabledChannels.ws;
    if (typeof raw.enabledChannels.sse === "boolean") enabledChannels.sse = raw.enabledChannels.sse;
  }
  const capabilities = { formSubmissions: true, dev: false };
  if (isRecord(raw.capabilities)) {
    if (typeof raw.capabilities.formSubmissions === "boolean") capabilities.formSubmissions = raw.capabilities.formSubmissions;
    if (typeof raw.capabilities.dev === "boolean") capabilities.dev = raw.capabilities.dev;
  }
  const contract = {
    siteId: typeof raw.siteId === "string" ? raw.siteId : "unknown",
    environment: typeof raw.environment === "string" ? raw.environment : "prod",
    version: typeof raw.version === "string" ? raw.version : "0",
    locale: typeof raw.locale === "string" ? raw.locale : "ru",
    protocols: providers,
    operations,
    endpoints,
    routes,
    themes,
    enabledChannels,
    capabilities
  };
  if (isRecord(raw.fallback)) {
    contract.fallback = {
      id: typeof raw.fallback.id === "string" ? raw.fallback.id : "",
      ...typeof raw.fallback.definition === "string" ? { definition: raw.fallback.definition } : {},
      ...isRecord(raw.fallback.params) ? { params: raw.fallback.params } : {}
    };
  }
  return { contract, providers, operations, endpoints, routes, themes };
}

// ../../../ui-runtime/src/core/form.ts
var EMAIL_RE = /^[^\s@]+@[^\s@]+\.[^\s@]+$/;
var FormRuntime = class {
  constructor(store, api, opts) {
    this.store = store;
    this.api = api;
    this.opts = opts;
  }
  async load(formId) {
    const existing = this.store.getState().forms[formId]?.definition;
    if (existing) return existing;
    const definition = await this.api.query("form.get", { formId });
    this.store.getState().setFormState(formId, { definition });
    return definition;
  }
  fields(formId) {
    return this.definitionOf(formId).fields;
  }
  /** Клиентская валидация по схеме из `form.get`. */
  validate(formId, values) {
    const def = this.definitionOf(formId);
    const errors = [];
    for (const field of def.fields) {
      errors.push(...validateField(field, values[field.name]));
    }
    for (const cross of def.validation ?? []) {
      errors.push(...validateCrossField(cross, values));
    }
    return { valid: errors.length === 0, errors };
  }
  /** Submit: клиентская валидация → `callEndpoint(form.submit)` с raw JSON. */
  async submit(formId, values) {
    let def = this.store.getState().forms[formId]?.definition;
    if (!def) def = await this.load(formId);
    const validated = this.validate(formId, values);
    const errorsMap = toMap(validated.errors);
    if (!validated.valid) {
      this.store.getState().setFormState(formId, { status: "error", values, errors: errorsMap });
      throw new FormValidationError(`\u0424\u043E\u0440\u043C\u0430 '${formId}' \u043D\u0435 \u043F\u0440\u043E\u0448\u043B\u0430 \u043A\u043B\u0438\u0435\u043D\u0442\u0441\u043A\u0443\u044E \u0432\u0430\u043B\u0438\u0434\u0430\u0446\u0438\u044E`);
    }
    this.store.getState().setFormState(formId, { status: "submitting", values, errors: {} });
    const locale = this.opts?.localeProvider ? this.opts.localeProvider() : this.store.getState().locale;
    const submittedAt = this.opts?.submittedAt ? this.opts.submittedAt() : (/* @__PURE__ */ new Date()).toISOString();
    const payload = { formId, locale, submittedAt, values };
    try {
      const res = await this.api.callEndpoint("form.submit", payload);
      this.store.getState().setFormState(formId, { status: "submitted", values, errors: {} });
      return res;
    } catch (err) {
      this.store.getState().setFormState(formId, { status: "error", values, errors: errorsMap });
      throw err;
    }
  }
  reset(formId) {
    this.store.getState().setFormState(formId, { status: "idle", values: {}, errors: {} });
  }
  definitionOf(formId) {
    const def = this.store.getState().forms[formId]?.definition;
    if (!def) {
      throw new DescriptorValidationError(`\u041E\u043F\u0440\u0435\u0434\u0435\u043B\u0435\u043D\u0438\u0435 \u0444\u043E\u0440\u043C\u044B '${formId}' \u043D\u0435 \u0437\u0430\u0433\u0440\u0443\u0436\u0435\u043D\u043E (form.get)`);
    }
    return def;
  }
};
function validateField(field, value) {
  const errors = [];
  const empty = value === void 0 || value === null || value === "";
  if (field.required && empty) {
    errors.push({ fieldId: field.name, ruleId: "required", message: `\u041F\u043E\u043B\u0435 '${field.name}' \u043E\u0431\u044F\u0437\u0430\u0442\u0435\u043B\u044C\u043D\u043E` });
    return errors;
  }
  if (empty) return errors;
  if (field.type === "email" && typeof value === "string" && !EMAIL_RE.test(value)) {
    errors.push({ fieldId: field.name, ruleId: "email", message: `\u041D\u0435\u0432\u0435\u0440\u043D\u044B\u0439 email \u0432 \u043F\u043E\u043B\u0435 '${field.name}'` });
  }
  if (field.type === "select" && field.options && field.options.length > 0 && !field.options.includes(String(value))) {
    errors.push({ fieldId: field.name, ruleId: "option", message: `\u041D\u0435\u0434\u043E\u043F\u0443\u0441\u0442\u0438\u043C\u043E\u0435 \u0437\u043D\u0430\u0447\u0435\u043D\u0438\u0435 \u0432 \u043F\u043E\u043B\u0435 '${field.name}'` });
  }
  for (const rule of field.rules ?? []) {
    const check = checkRule(rule.id, value, rule.value);
    if (check) {
      errors.push({
        fieldId: field.name,
        ruleId: rule.id,
        message: rule.message ?? `\u041F\u043E\u043B\u0435 '${field.name}' \u043D\u0435 \u043F\u0440\u043E\u0448\u043B\u043E \u043F\u0440\u043E\u0432\u0435\u0440\u043A\u0443 '${rule.id}'`
      });
    }
  }
  return errors;
}
function checkRule(id, value, expected) {
  switch (id) {
    case "minLength":
      return typeof value === "string" && typeof expected === "number" && value.length < expected;
    case "maxLength":
      return typeof value === "string" && typeof expected === "number" && value.length > expected;
    case "min":
      return typeof value === "number" && typeof expected === "number" && value < expected;
    case "max":
      return typeof value === "number" && typeof expected === "number" && value > expected;
    case "pattern": {
      if (typeof value !== "string") return false;
      const re = expected instanceof RegExp ? expected : new RegExp(String(expected));
      return !re.test(value);
    }
    default:
      return false;
  }
}
function validateCrossField(rule, values) {
  if (rule.id !== "confirmMatch" || rule.fields.length < 2) return [];
  const [first, ...rest] = rule.fields;
  const base = String(values[first] ?? "");
  const differs = rest.some((f) => String(values[f] ?? "") !== base);
  if (!differs) return [];
  return [{ fieldId: rest[0], ruleId: "confirmMatch", message: rule.message ?? "\u0417\u043D\u0430\u0447\u0435\u043D\u0438\u044F \u043F\u043E\u043B\u0435\u0439 \u043D\u0435 \u0441\u043E\u0432\u043F\u0430\u0434\u0430\u044E\u0442" }];
}
function toMap(errors) {
  const out = {};
  for (const e of errors) {
    (out[e.fieldId] ??= []).push(e.message ?? e.ruleId);
  }
  return out;
}

// ../../../ui-runtime/src/core/i18n.ts
function normalize(locale) {
  return locale.trim().split(/[-_]/)[0].toLowerCase();
}
var I18n = class {
  constructor(opts, store, env) {
    this.opts = opts;
    this.store = store;
    this.env = env;
    this.listeners = /* @__PURE__ */ new Set();
    this.storageKey = opts.storageKey ?? "liapoldus.locale";
  }
  /** localStorage → navigator.language → defaultLocale. */
  detect() {
    const stored = this.env?.storage?.getItem(this.storageKey) ?? null;
    const candidate = stored ?? this.env?.navigatorLanguage ?? this.opts.defaultLocale;
    const locale = this.resolveLocale(candidate);
    this.store.getState().setLocale(locale);
    if (stored !== locale) this.env?.storage?.setItem(this.storageKey, locale);
    this.notify(locale);
  }
  getLocale() {
    return this.store.getState().locale;
  }
  /** normalize + сохранить + уведомить + переспрос контента. */
  async setLocale(locale) {
    const resolved = this.resolveLocale(locale);
    const prev = this.store.getState().locale;
    this.store.getState().setLocale(resolved);
    this.env?.storage?.setItem(this.storageKey, resolved);
    if (prev !== resolved) this.notify(resolved);
    await this.opts.syncContent?.(resolved);
  }
  /** 'ru-RU' → 'ru'; при supportedLocales: совпавший → нормализованный, несовпавший → defaultLocale (strict — бросок). */
  resolveLocale(candidate) {
    const normalized = normalize(candidate);
    const supported = this.opts.supportedLocales ?? [];
    if (supported.length === 0) return normalized;
    const found = supported.find((s) => normalize(s) === normalized);
    if (found) return normalize(found);
    if (this.opts.strict) throw new LocaleUnsupportedError(`\u041B\u043E\u043A\u0430\u043B\u044C '${candidate}' \u043D\u0435 \u043F\u043E\u0434\u0434\u0435\u0440\u0436\u0438\u0432\u0430\u0435\u0442\u0441\u044F`);
    return this.opts.defaultLocale;
  }
  /** Подписка на смену локали; возвращаемая функция отписывает. */
  onLocaleChange(cb) {
    this.listeners.add(cb);
    return () => {
      this.listeners.delete(cb);
    };
  }
  /** Строка по dot-path в `store.content`; отсутствует → сырой ключ. */
  t(key, params) {
    const value = lookupByDot(this.store.getState().content, key);
    if (typeof value !== "string") return key;
    return interpolate(value, params);
  }
  /** Плоский словарь всех строк текущего кантента (ключ — dot-path). */
  strings() {
    const out = {};
    for (const [id, entry] of Object.entries(this.store.getState().content)) {
      flatten(id, entry, out);
    }
    return out;
  }
  notify(locale) {
    for (const cb of this.listeners) cb(locale);
  }
};
function lookupByDot(content, key) {
  const [head, ...path] = key.split(".");
  let value = content[head];
  for (const part of path) {
    if (value === null || typeof value !== "object") return void 0;
    value = value[part];
  }
  return value;
}
function flatten(prefix, value, out) {
  if (typeof value === "string") {
    out[prefix] = value;
    return;
  }
  if (Array.isArray(value) || value !== null && typeof value === "object") {
    for (const [k, v] of Object.entries(value)) {
      flatten(`${prefix}.${k}`, v, out);
    }
  }
}
function interpolate(template, params) {
  if (!params) return template;
  return template.replace(/\{([^}]+)\}/g, (m, name) => {
    const v = params[name];
    return v === void 0 ? m : String(v);
  });
}

// ../../../ui-runtime/src/core/registry.ts
var RuntimeRegistry = class {
  constructor() {
    this.providers = /* @__PURE__ */ new Map();
    this.operations = /* @__PURE__ */ new Map();
    this.endpoints = /* @__PURE__ */ new Map();
    this.routes = [];
    this.themes = [];
  }
  register(descriptor, _ctx) {
    switch (this.kindOf(descriptor)) {
      case "provider":
        this.registerProvider(descriptor);
        return;
      case "operation":
        this.registerOperation(descriptor);
        return;
      case "endpoint":
        this.registerEndpoint(descriptor);
        return;
      case "route":
        this.registerRoute(descriptor);
        return;
      case "theme":
        this.registerTheme(descriptor);
        return;
      default:
        throw new DescriptorValidationError(`\u041D\u0435\u0438\u0437\u0432\u0435\u0441\u0442\u043D\u044B\u0439 \u0442\u0438\u043F \u0434\u0435\u0441\u043A\u0440\u0438\u043F\u0442\u043E\u0440\u0430: ${String(descriptor?.kind)}`);
    }
  }
  /** kind из `kind`-поля; канонические route/theme без kind распознаются структурно. */
  kindOf(d) {
    if (d && typeof d === "object") {
      const rec = d;
      if (typeof rec.kind === "string") return rec.kind;
      if ("action" in rec && "matcher" in rec) return "route";
      if ("themeId" in rec && "tokens" in rec) return "theme";
    }
    return "unknown";
  }
  registerProvider(descriptor) {
    if (this.providers.has(descriptor.id)) {
      throw new DuplicateRegistrationError(`Provider '${descriptor.id}' \u0443\u0436\u0435 \u0437\u0430\u0440\u0435\u0433\u0438\u0441\u0442\u0440\u0438\u0440\u043E\u0432\u0430\u043D`);
    }
    if (descriptor.baseUrl !== void 0) {
      if (typeof descriptor.baseUrl !== "string" || !/^[a-z][a-z0-9+.-]*:\/\//i.test(descriptor.baseUrl)) {
        throw new DescriptorValidationError(
          `baseUrl '${String(descriptor.baseUrl)}' \u043D\u0435 \u044F\u0432\u043B\u044F\u0435\u0442\u0441\u044F \u0432\u0430\u043B\u0438\u0434\u043D\u044B\u043C \u0430\u0431\u0441\u043E\u043B\u044E\u0442\u043D\u044B\u043C URL`,
          { entityId: descriptor.id, path: "baseUrl" }
        );
      }
      let url;
      try {
        url = new URL(descriptor.baseUrl);
      } catch {
        throw new DescriptorValidationError(
          `baseUrl '${descriptor.baseUrl}' \u043D\u0435 \u044F\u0432\u043B\u044F\u0435\u0442\u0441\u044F \u0432\u0430\u043B\u0438\u0434\u043D\u044B\u043C URL`,
          { entityId: descriptor.id, path: "baseUrl" }
        );
      }
      if (!/^(https?|wss?):$/.test(url.protocol)) {
        throw new DescriptorValidationError(
          `baseUrl '${descriptor.baseUrl}' \u0438\u043C\u0435\u0435\u0442 \u043D\u0435\u043F\u043E\u0434\u0434\u0435\u0440\u0436\u0438\u0432\u0430\u0435\u043C\u044B\u0439 \u043F\u0440\u043E\u0442\u043E\u043A\u043E\u043B`,
          { entityId: descriptor.id, path: "baseUrl" }
        );
      }
    }
    const provider = {
      id: descriptor.id,
      protocol: descriptor.protocol,
      state: { registered: true }
    };
    if (descriptor.baseUrl !== void 0) provider.baseUrl = descriptor.baseUrl;
    if (descriptor.defaults) provider.defaults = descriptor.defaults;
    if (descriptor.subscribe) provider.subscribe = descriptor.subscribe;
    this.providers.set(descriptor.id, provider);
  }
  getProvider(id) {
    return this.providers.get(id) ?? null;
  }
  hasProvider(id) {
    return this.providers.has(id);
  }
  get providersList() {
    return Array.from(this.providers.values());
  }
  registerOperation(descriptor) {
    if (this.operations.has(descriptor.id)) {
      throw new DuplicateRegistrationError(`Operation '${descriptor.id}' \u0443\u0436\u0435 \u0437\u0430\u0440\u0435\u0433\u0438\u0441\u0442\u0440\u0438\u0440\u043E\u0432\u0430\u043D\u0430`);
    }
    const provider = this.providers.get(descriptor.providerId);
    if (!provider) {
      throw new UnknownProviderError(`Provider '${descriptor.providerId}' \u0434\u043B\u044F operation '${descriptor.id}' \u043D\u0435 \u0437\u0430\u0440\u0435\u0433\u0438\u0441\u0442\u0440\u0438\u0440\u043E\u0432\u0430\u043D`);
    }
    if (descriptor.scope === void 0) descriptor.scope = "public";
    if (descriptor.scope !== "public" && descriptor.scope !== "server") {
      throw new DescriptorValidationError("scope operation \u0434\u043E\u043B\u0436\u0435\u043D \u0431\u044B\u0442\u044C public|server", { entityId: descriptor.id, path: "scope" });
    }
    if (descriptor.poll) {
      if (descriptor.poll.schedule) parseCron(descriptor.poll.schedule);
    }
    if (provider.protocol !== "http" && descriptor.path !== void 0) {
      const subscribe = descriptor.subscribe;
      const url = subscribe?.url ?? provider.subscribe?.url;
      if (provider.protocol !== "graphql" && !url) {
        throw new DescriptorValidationError(
          `Operation '${descriptor.id}' (${provider.protocol}) \u0442\u0440\u0435\u0431\u0443\u0435\u0442 subscribe.url`,
          { entityId: descriptor.id, path: "subscribe.url" }
        );
      }
    }
    const op = {
      id: descriptor.id,
      provider,
      typeOp: descriptor.typeOp,
      cache: descriptor.cache,
      scope: descriptor.scope,
      state: { registered: true }
    };
    if (descriptor.method) op.method = descriptor.method;
    if (descriptor.path) op.path = descriptor.path;
    if (descriptor.params) op.params = descriptor.params;
    if (descriptor.input) op.input = descriptor.input;
    if (descriptor.output) op.output = descriptor.output;
    if (descriptor.type) op.type = descriptor.type;
    if (typeof descriptor.ttl === "number") op.ttl = descriptor.ttl;
    if (descriptor.poll) op.poll = descriptor.poll;
    if (descriptor.push === true) op.push = true;
    if (descriptor.subscribe) op.subscribe = descriptor.subscribe;
    if (descriptor.headers) op.headers = descriptor.headers;
    this.operations.set(descriptor.id, op);
  }
  getOperation(id) {
    return this.operations.get(id) ?? null;
  }
  hasOperation(id) {
    return this.operations.has(id);
  }
  getOperationFor(typeOp, providerId) {
    for (const op of this.operations.values()) {
      if (op.typeOp === typeOp && op.provider.id === providerId) return op;
    }
    return null;
  }
  /** Ищет по чистому id, либо по бейджеванному `id#query` (§12a). */
  findOperationByIdOrBadge(ref) {
    const direct = this.operations.get(ref);
    if (direct) return direct;
    const { name, badge } = splitBadgedId(ref);
    if (badge) {
      for (const op of this.operations.values()) {
        if (op.id === name && op.typeOp === badge) return op;
      }
    }
    return null;
  }
  resolve(typeOp, ref) {
    const op = this.operations.get(ref) ?? this.getOperationFor(typeOp, ref) ?? this.findOperationByIdOrBadge(ref);
    if (!op) {
      const provider = this.providers.get(ref);
      const forType = provider ? this.getOperationFor(typeOp, ref) : null;
      if (forType) return forType;
      throw new UnknownEntityError(
        `Operation '${ref}' \u0442\u0438\u043F\u0430 '${typeOp}' \u043D\u0435 \u043D\u0430\u0439\u0434\u0435\u043D\u0430` + (provider ? ` \u0434\u043B\u044F provider '${ref}'` : "")
      );
    }
    if (op.typeOp !== typeOp) {
      throw new DescriptorValidationError(
        `Operation '${op.id}' \u0438\u043C\u0435\u0435\u0442 \u0442\u0438\u043F '${op.typeOp}', \u0430 \u0442\u0440\u0435\u0431\u0443\u0435\u0442\u0441\u044F '${typeOp}'`,
        { entityId: op.id, path: "typeOp" }
      );
    }
    return op;
  }
  get operationsList() {
    return Array.from(this.operations.values());
  }
  registerEndpoint(descriptor) {
    if (this.endpoints.has(descriptor.id)) {
      throw new DuplicateRegistrationError(`Endpoint '${descriptor.id}' \u0443\u0436\u0435 \u0437\u0430\u0440\u0435\u0433\u0438\u0441\u0442\u0440\u0438\u0440\u043E\u0432\u0430\u043D`);
    }
    if (!descriptor.path || descriptor.path.length === 0) {
      throw new DescriptorValidationError("endpoint \u0442\u0440\u0435\u0431\u0443\u0435\u0442 path", { entityId: descriptor.id, path: "path" });
    }
    if (!descriptor.method) {
      throw new DescriptorValidationError("endpoint \u0442\u0440\u0435\u0431\u0443\u0435\u0442 method", { entityId: descriptor.id, path: "method" });
    }
    const operation = this.operations.get(descriptor.operationId);
    if (!operation) {
      throw new UnknownEntityError(`Operation '${descriptor.operationId}' \u0434\u043B\u044F endpoint '${descriptor.id}' \u043D\u0435 \u043D\u0430\u0439\u0434\u0435\u043D\u0430`);
    }
    if (operation.scope !== "server") {
      throw new DescriptorValidationError(
        `Endpoint '${descriptor.id}' \u0441\u0441\u044B\u043B\u0430\u0435\u0442\u0441\u044F \u043D\u0430 \u043F\u0443\u0431\u043B\u0438\u0447\u043D\u0443\u044E operation '${descriptor.operationId}' \u2014 endpoint \u0442\u0440\u0435\u0431\u0443\u0435\u0442 server-\u043E\u043F\u0435\u0440\u0430\u0446\u0438\u044E`,
        { entityId: descriptor.id, path: "operationId" }
      );
    }
    const endpoint = {
      id: descriptor.id,
      path: descriptor.path,
      method: descriptor.method,
      operation,
      state: { registered: true }
    };
    if (descriptor.input) endpoint.input = descriptor.input;
    if (descriptor.output) endpoint.output = descriptor.output;
    this.endpoints.set(descriptor.id, endpoint);
  }
  getEndpoint(id) {
    return this.endpoints.get(id) ?? null;
  }
  hasEndpoint(id) {
    return this.endpoints.has(id);
  }
  get endpointsList() {
    return Array.from(this.endpoints.values());
  }
  registerRoute(descriptor) {
    const existing = this.routes.find((r) => r.id === descriptor.id);
    if (existing) {
      throw new DuplicateRegistrationError(`Route '${descriptor.id}' \u0443\u0436\u0435 \u0437\u0430\u0440\u0435\u0433\u0438\u0441\u0442\u0440\u0438\u0440\u043E\u0432\u0430\u043D\u0430`);
    }
    this.routes.push(descriptor);
  }
  get routesList() {
    return this.routes.slice();
  }
  registerTheme(descriptor) {
    const existing = this.themes.find((t) => t.themeId === descriptor.themeId);
    if (existing) {
      throw new DuplicateRegistrationError(`Theme '${descriptor.themeId}' \u0443\u0436\u0435 \u0437\u0430\u0440\u0435\u0433\u0438\u0441\u0442\u0440\u0438\u0440\u043E\u0432\u0430\u043D\u0430`);
    }
    this.themes.push(descriptor);
  }
  hasTheme(themeId) {
    return this.themes.some((t) => t.themeId === themeId);
  }
  get themesList() {
    return this.themes.slice();
  }
  /** Очищает все регистрации (§12a drop). */
  drop() {
    this.providers.clear();
    this.operations.clear();
    this.endpoints.clear();
    this.routes = [];
    this.themes = [];
  }
};

// ../../../ui-runtime/src/core/matcher.ts
function splitPath(path) {
  const qIdx = path.indexOf("?");
  const pathname = qIdx === -1 ? path : path.slice(0, qIdx);
  const query = {};
  if (qIdx !== -1) {
    for (const pair of path.slice(qIdx + 1).split("&")) {
      if (!pair) continue;
      const eq = pair.indexOf("=");
      const key = eq === -1 ? pair : pair.slice(0, eq);
      const value = eq === -1 ? "" : pair.slice(eq + 1);
      try {
        query[decodeURIComponent(key)] = decodeURIComponent(value);
      } catch {
        query[key] = value;
      }
    }
  }
  return { pathname, query };
}
function applyTargetTemplate(target, matcherSource, pathname) {
  return target.replace(/\$(\d+)/g, (_, idx) => {
    const re = new RegExp(matcherSource);
    const m = re.exec(pathname);
    const n = Number(idx);
    return m && m[n] !== void 0 ? m[n] : "";
  });
}
function appendQuery(target, query) {
  const keys = Object.keys(query);
  if (keys.length === 0) return target;
  const qs = keys.map((k) => `${encodeURIComponent(k)}=${encodeURIComponent(query[k])}`).join("&");
  return target.includes("?") ? `${target}&${qs}` : `${target}?${qs}`;
}
function createMatcher(routes) {
  const ordered = [...routes].sort((a, b) => b.priority - a.priority);
  return (path) => {
    const { pathname, query } = splitPath(path);
    for (const route of ordered) {
      const re = new RegExp(route.matcher);
      const m = re.exec(pathname);
      if (m === null) continue;
      const params = {};
      if (m.groups) {
        for (const [k, v] of Object.entries(m.groups)) {
          if (v !== void 0) params[k] = v;
        }
      }
      return { route, params, query };
    }
    return null;
  };
}

// ../../../ui-runtime/src/core/router.ts
var MAX_REDIRECT_DEPTH = 10;
var Router = class {
  constructor(store, env) {
    this.store = store;
    this.env = env;
    this.routes = [];
    this.listeners = /* @__PURE__ */ new Map();
  }
  addRoutes(descriptors) {
    this.routes.push(...descriptors);
  }
  replaceRoutes(descriptors) {
    this.routes = descriptors.slice();
  }
  /** Та же функция матчинга, что у edge (общий createMatcher). */
  match(path) {
    return createMatcher(this.routes)(path);
  }
  on(event, listener) {
    const list = this.listeners.get(event) ?? [];
    list.push(listener);
    this.listeners.set(event, list);
    return () => {
      this.listeners.set(
        event,
        (this.listeners.get(event) ?? []).filter((l) => l !== listener)
      );
    };
  }
  navigate(to) {
    const resolved = this.match(to);
    if (!resolved) {
      throw new RouteNotFoundError(`\u041C\u0430\u0440\u0448\u0440\u0443\u0442 \u043D\u0435 \u043D\u0430\u0439\u0434\u0435\u043D: ${to}`);
    }
    this.dispatch(resolved, to, 0);
  }
  emit(event) {
    for (const listener of this.listeners.get(event.type) ?? []) listener(event);
  }
  dispatch(resolved, to, depth) {
    if (depth > MAX_REDIRECT_DEPTH) {
      throw new RouteNotFoundError(`\u0421\u043B\u0438\u0448\u043A\u043E\u043C \u043C\u043D\u043E\u0433\u043E \u0440\u0435\u0434\u0438\u0440\u0435\u043A\u0442\u043E\u0432 \u043F\u0440\u0438 \u043D\u0430\u0432\u0438\u0433\u0430\u0446\u0438\u0438: ${to}`);
    }
    const action = resolved.route.action;
    if (action.type === "renderPage") {
      this.store.getState().setRoute(resolved);
      this.emit({ type: "route", route: resolved });
      this.emit({ type: "renderPage", pageId: action.pageId });
      return;
    }
    if (action.type === "serveAsset") {
      this.emit({ type: "external", url: to });
      this.env?.window?.location.assign(to);
      return;
    }
    const { pathname, query } = splitPath(to);
    const target = applyTargetTemplate(action.target, resolved.route.matcher, pathname);
    const url = action.keepQuery ? appendQuery(target, query) : target;
    const next = this.match(url);
    if (!next) {
      throw new RouteNotFoundError(`\u0426\u0435\u043B\u044C \u0440\u0435\u0434\u0438\u0440\u0435\u043A\u0442\u0430 \u043D\u0435 \u043D\u0430\u0439\u0434\u0435\u043D\u0430: ${url}`);
    }
    this.dispatch(next, url, depth + 1);
  }
};

// ../../../ui-runtime/node_modules/zustand/esm/vanilla.mjs
var createStoreImpl = (createState) => {
  let state;
  const listeners = /* @__PURE__ */ new Set();
  const setState = (partial, replace) => {
    const nextState = typeof partial === "function" ? partial(state) : partial;
    if (!Object.is(nextState, state)) {
      const previousState = state;
      state = (replace != null ? replace : typeof nextState !== "object" || nextState === null) ? nextState : Object.assign({}, state, nextState);
      listeners.forEach((listener) => listener(state, previousState));
    }
  };
  const getState = () => state;
  const getInitialState = () => initialState;
  const subscribe = (listener) => {
    listeners.add(listener);
    return () => listeners.delete(listener);
  };
  const destroy = () => {
    if ((import.meta.env ? import.meta.env.MODE : void 0) !== "production") {
      console.warn(
        "[DEPRECATED] The `destroy` method will be unsupported in a future version. Instead use unsubscribe function returned by subscribe. Everything will be garbage-collected if store is garbage-collected."
      );
    }
    listeners.clear();
  };
  const api = { setState, getState, getInitialState, subscribe, destroy };
  const initialState = state = createState(setState, getState, api);
  return api;
};
var createStore = (createState) => createState ? createStoreImpl(createState) : createStoreImpl;

// ../../../ui-runtime/src/core/store.ts
var defaultStoreState = {
  ready: false,
  tree: null,
  routes: [],
  route: null,
  tokens: {},
  content: {},
  assets: {},
  operationResults: {},
  locale: "",
  forms: {}
};
var idleForm = { status: "idle", values: {}, errors: {} };
function createRuntimeStore(initial) {
  const store = createStore()((set) => ({
    ...defaultStoreState,
    ...initial,
    setReady: (ready = true) => set({ ready }),
    setTree: (tree) => set({ tree }),
    setRoutes: (routes) => set({ routes }),
    setRoute: (route) => set({ route }),
    applyTokens: (tokens) => set({ tokens }),
    setContent: (content) => set({ content }),
    setAssets: (assets) => set({ assets }),
    setOperationResult: (key, value) => set((s) => ({ operationResults: { ...s.operationResults, [key]: value } })),
    setLocale: (locale) => set({ locale }),
    setFormState: (formId, patch) => set((s) => ({
      forms: {
        ...s.forms,
        [formId]: { ...idleForm, ...s.forms[formId], ...patch }
      }
    }))
  }));
  const api = store;
  api.subscribeSlice = (selector, listener) => {
    let prev = selector(store.getState());
    return store.subscribe((state, prevState) => {
      const next = selector(state);
      if (next !== prev) {
        listener(next, prev);
        prev = next;
      }
      void prevState;
    });
  };
  return api;
}

// ../../../ui-runtime/src/core/tokens.ts
function asVar(name) {
  return name.startsWith("--") ? name : `--${name}`;
}
function isDef(v) {
  if (typeof v === "string") return true;
  if (typeof v !== "object" || v === null || Array.isArray(v)) return false;
  if ("value" in v) return typeof v.value === "string";
  if ("ref" in v) return typeof v.ref === "string";
  return false;
}
function resolveValue(name, def, tokens) {
  if (typeof def === "string") return def;
  if ("value" in def) return def.value;
  const refName = asVar(def.ref);
  let entry;
  for (const [k, v] of Object.entries(tokens)) {
    if (asVar(k) === refName) {
      entry = [k, v];
      break;
    }
  }
  if (!entry) {
    throw new DescriptorValidationError(`\u0422\u043E\u043A\u0435\u043D '${name}' \u0441\u0441\u044B\u043B\u0430\u0435\u0442\u0441\u044F \u043D\u0430 \u043E\u0442\u0441\u0443\u0442\u0441\u0442\u0432\u0443\u044E\u0449\u0438\u0439 \u0442\u043E\u043A\u0435\u043D '${def.ref}'`);
  }
  if (entry[0] === name || asVar(entry[0]) === asVar(name)) {
    throw new DescriptorValidationError(`\u0422\u043E\u043A\u0435\u043D '${name}' \u0441\u0441\u044B\u043B\u0430\u0435\u0442\u0441\u044F \u0441\u0430\u043C \u043D\u0430 \u0441\u0435\u0431\u044F`);
  }
  return resolveValue(name, entry[1], tokens);
}
function renderCss(scope, flat) {
  const body = [...flat.entries()].map(([k, v]) => `${k}: ${v};`).join(" ");
  return `${scope} { ${body} }`;
}
var DesignTokens = class {
  constructor(opts) {
    this.current = /* @__PURE__ */ new Map();
    this.scope = opts?.scope ?? ":root";
    this.env = opts?.env;
  }
  apply(theme) {
    const flat = /* @__PURE__ */ new Map();
    for (const [name, def] of Object.entries(theme.tokens)) {
      if (!isDef(def)) {
        throw new DescriptorValidationError(
          `\u0422\u043E\u043A\u0435\u043D '${name}' \u0442\u0435\u043C\u044B '${theme.themeId}' \u0434\u043E\u043B\u0436\u0435\u043D \u0431\u044B\u0442\u044C string | { value } | { ref }`
        );
      }
      flat.set(asVar(name), resolveValue(asVar(name), def, theme.tokens));
    }
    this.current = flat;
    this.env?.writeCss?.(renderCss(this.scope, flat));
    if (theme.fonts?.length) {
      this.attachFonts(theme.fonts);
    }
  }
  get(name) {
    return this.current.get(asVar(name));
  }
  /** Плоский словарь `--имя → значение` применённой темы (для синхронизации со store). */
  values() {
    return Object.fromEntries(this.current);
  }
  attachFonts(fonts) {
    this.env?.onFonts?.(fonts);
    const doc = this.env?.document;
    if (!doc) return;
    for (const font of fonts) {
      const link = doc.createElement("link");
      link.rel = "stylesheet";
      link.type = "text/css";
      link.href = font;
      doc.head.appendChild(link);
    }
  }
};

// ../../../ui-runtime/src/core/transport/compound.ts
var CompoundTransport = class {
  constructor(factory) {
    this.factory = factory;
  }
  request(req) {
    return this.factory.create(req.provider).request(req);
  }
  subscribe(req, onData, onError) {
    const transport = this.factory.create(req.provider);
    if (typeof transport.subscribe !== "function") {
      throw new Error(`\u0422\u0440\u0430\u043D\u0441\u043F\u043E\u0440\u0442 \u043F\u0440\u043E\u0432\u0430\u0439\u0434\u0435\u0440\u0430 '${req.provider.id}' \u043D\u0435 \u043F\u043E\u0434\u0434\u0435\u0440\u0436\u0438\u0432\u0430\u0435\u0442 push-\u043A\u0430\u043D\u0430\u043B`);
    }
    return transport.subscribe(req, onData, onError);
  }
  close() {
    this.factory.clear();
  }
};

// ../../../ui-runtime/src/core/transport/graphql.ts
var DEFAULT_TIMEOUT_MS = 3e4;
var GraphqlTransport = class {
  constructor(provider, env) {
    this.provider = provider;
    this.fetcher = env?.fetch ?? globalThis.fetch;
    if (!this.fetcher) throw new Error("GraphqlTransport \u0442\u0440\u0435\u0431\u0443\u0435\u0442 fetch");
  }
  buildUrl() {
    const base = this.provider.baseUrl ?? "/graphql";
    return base;
  }
  async request(req) {
    const query = req.gql;
    if (!query) {
      throw new TransportError(`GraphQL-\u0437\u0430\u043F\u0440\u043E\u0441 \u0434\u043B\u044F '${req.operation.id}' \u0442\u0440\u0435\u0431\u0443\u0435\u0442 gql-\u0441\u0442\u0440\u043E\u043A\u0443`);
    }
    const url = this.buildUrl();
    const timeoutMs = req.timeoutMs ?? this.provider.defaults?.timeoutMs ?? DEFAULT_TIMEOUT_MS;
    const controller = new AbortController();
    let timedOut = false;
    const timer = timeoutMs > 0 ? setTimeout(() => {
      timedOut = true;
      controller.abort();
    }, timeoutMs) : void 0;
    const abortPromise = new Promise((_, reject) => {
      controller.signal.addEventListener("abort", () => {
        reject(
          new TransportError(
            timedOut ? `\u0422\u0430\u0439\u043C\u0430\u0443\u0442 GraphQL-\u0437\u0430\u043F\u0440\u043E\u0441\u0430 (${timeoutMs}\u043C\u0441)` : "GraphQL-\u0437\u0430\u043F\u0440\u043E\u0441 \u043F\u0440\u0435\u0440\u0432\u0430\u043D"
          )
        );
      });
    });
    const headers = { "Content-Type": "application/json" };
    if (this.provider.defaults?.headers) Object.assign(headers, this.provider.defaults.headers);
    if (req.headers) Object.assign(headers, req.headers);
    try {
      const body = JSON.stringify({ query, variables: req.input ?? {} });
      const fetchPromise = this.fetcher(url, {
        method: "POST",
        headers,
        body,
        signal: controller.signal
      });
      const res = await Promise.race([fetchPromise, abortPromise]);
      if (!res.ok) {
        throw new TransportError(`HTTP ${res.status} \u0434\u043B\u044F GraphQL`, { status: res.status, httpError: true });
      }
      const payload = await res.json().catch(() => null);
      if (payload && Array.isArray(payload.errors) && payload.errors.length > 0) {
        throw new TransportError("GraphQL \u0432\u0435\u0440\u043D\u0443\u043B \u043E\u0448\u0438\u0431\u043A\u0438", { graphqlErrors: payload.errors });
      }
      return { status: res.status, ok: true, body: payload?.data ?? payload };
    } catch (err) {
      if (err instanceof TransportError) throw err;
      throw new TransportError(`\u041E\u0448\u0438\u0431\u043A\u0430 GraphQL: ${err instanceof Error ? err.message : String(err)}`);
    } finally {
      if (timer) clearTimeout(timer);
    }
  }
};

// ../../../ui-runtime/src/core/transport/http.ts
var DEFAULT_TIMEOUT_MS2 = 3e4;
function substitutePath(template, params) {
  return template.replace(/:([A-Za-z0-9_-]+)/g, (_m, name) => String(params[name] ?? "")).replace(/\{([A-Za-z0-9_-]+)\}/g, (_m, name) => String(params[name] ?? ""));
}
function buildQueryString(params) {
  const parts = [];
  const push = (key, value) => {
    if (value === void 0 || value === null) return;
    if (Array.isArray(value)) {
      for (const v of value) push(key, v);
      return;
    }
    parts.push(`${encodeURIComponent(key)}=${encodeURIComponent(String(value))}`);
  };
  for (const [k, v] of Object.entries(params)) push(k, v);
  return parts.length > 0 ? `?${parts.join("&")}` : "";
}
function buildHttpUrl(provider, operation, params) {
  const pathTemplate = operation.path ?? "";
  const path = substitutePath(pathTemplate, params);
  let base = "";
  if (provider.baseUrl !== void 0) base = provider.baseUrl.replace(/\/$/, "");
  const fullPath = `${base}${path}`;
  const queryParams = {};
  const inParam = operation.params?.in;
  if (inParam !== "body") {
    const isGetLike = operation.method === void 0 || operation.method === "GET";
    if (inParam === "query" || inParam === void 0 && isGetLike) {
      for (const [k, v] of Object.entries(params)) {
        if (!pathTemplate.includes(`:${k}`) && !pathTemplate.includes(`{${k}}`)) queryParams[k] = v;
      }
    }
  }
  return `${fullPath}${buildQueryString(queryParams)}`;
}
function buildBody(operation, params) {
  if (operation.params?.in === "body") {
    return JSON.stringify(reduceParams(params) ?? {});
  }
  if (operation.method && operation.method !== "GET") {
    return JSON.stringify(reduceParams(params) ?? {});
  }
  return void 0;
}
function reduceParams(params) {
  if (!params) return void 0;
  if (params.input !== void 0) return params.input;
  if (params.body !== void 0) return params.body;
  return params;
}
function mergeHeaders(provider, operation, requestHeaders) {
  const out = {};
  if (provider.defaults?.headers) Object.assign(out, provider.defaults.headers);
  if (operation.headers) Object.assign(out, operation.headers);
  if (requestHeaders) Object.assign(out, requestHeaders);
  return out;
}
function originSafe(_fullUrl) {
  void _fullUrl;
}
var HttpTransport = class {
  constructor(provider, env) {
    this.provider = provider;
    this.fetcher = env?.fetch ?? globalThis.fetch;
    if (!this.fetcher) throw new Error("HttpTransport \u0442\u0440\u0435\u0431\u0443\u0435\u0442 fetch");
  }
  buildRequest(req) {
    const operation = req.operation;
    const params = req.params ?? {};
    const url = buildHttpUrl(this.provider, operation, params);
    const init = {
      method: operation.method ?? "GET",
      headers: mergeHeaders(this.provider, operation, req.headers)
    };
    const body = buildBody(operation, params);
    if (body !== void 0) init.body = body;
    return { url, init };
  }
  async request(req) {
    const { url, init } = this.buildRequest(req);
    const operation = req.operation;
    const timeoutMs = req.timeoutMs ?? this.provider.defaults?.timeoutMs ?? DEFAULT_TIMEOUT_MS2;
    const controller = new AbortController();
    let timedOut = false;
    const timer = timeoutMs > 0 ? setTimeout(() => {
      timedOut = true;
      controller.abort();
    }, timeoutMs) : void 0;
    const abortPromise = new Promise((_, reject) => {
      controller.signal.addEventListener("abort", () => {
        reject(
          new TransportError(
            timedOut ? `\u0422\u0430\u0439\u043C\u0430\u0443\u0442 \u0437\u0430\u043F\u0440\u043E\u0441\u0430 ${operation.id} (${timeoutMs}\u043C\u0441)` : `\u0417\u0430\u043F\u0440\u043E\u0441 ${operation.id} \u043F\u0440\u0435\u0440\u0432\u0430\u043D`,
            { message: timedOut ? "timeout" : "aborted" }
          )
        );
      });
    });
    try {
      const fetchPromise = this.fetcher(url, { ...init, signal: controller.signal });
      const res = await Promise.race([fetchPromise, abortPromise]);
      if (!res.ok) {
        throw new TransportError(`HTTP ${res.status} \u0434\u043B\u044F ${operation.id}`, {
          status: res.status,
          httpError: true
        });
      }
      let body = null;
      try {
        body = await res.json();
      } catch {
        const text = await res.text().catch(() => "");
        if (text.length > 0) {
          try {
            body = JSON.parse(text);
          } catch {
            body = text;
          }
        }
      }
      void originSafe(url);
      return { status: res.status, ok: true, body };
    } catch (err) {
      if (err instanceof TransportError) throw err;
      throw new TransportError(`\u041E\u0448\u0438\u0431\u043A\u0430 \u0442\u0440\u0430\u043D\u0441\u043F\u043E\u0440\u0442\u0430 \u0434\u043B\u044F ${operation.id}: ${err instanceof Error ? err.message : String(err)}`);
    } finally {
      if (timer) clearTimeout(timer);
    }
  }
};

// ../../../ui-runtime/src/core/transport/ws-sse.ts
function wsSubscribeUrl(operation) {
  const opUrl = operation.subscribe?.url;
  if (opUrl) return opUrl;
  const provUrl = operation.provider.subscribe?.url;
  if (provUrl) return provUrl;
  if (operation.provider.baseUrl) {
    return `${operation.provider.baseUrl.replace(/\/$/, "")}${operation.path ?? ""}`;
  }
  return void 0;
}
var WebSocketTransport = class {
  constructor(_provider, ctor) {
    this.ctor = ctor;
    this.socket = null;
  }
  async request(req) {
    const url = wsSubscribeUrl(req.operation);
    if (!url) {
      throw new TransportError(`\u041D\u0435\u0442 URL \u0434\u043B\u044F WebSocket-\u043A\u0430\u043D\u0430\u043B\u0430 \u043E\u043F\u0435\u0440\u0430\u0446\u0438\u0438 '${req.operation.id}'`);
    }
    const timeoutMs = req.timeoutMs ?? 3e4;
    return new Promise((resolve, reject) => {
      let settled = false;
      const ws = new this.ctor(url);
      this.socket = ws;
      const cleanup = () => {
        try {
          ws.close();
        } catch {
        }
        if (timer) clearTimeout(timer);
      };
      const timer = setTimeout(() => {
        if (!settled) {
          settled = true;
          cleanup();
          reject(new TransportError(`\u0422\u0430\u0439\u043C\u0430\u0443\u0442 \u043E\u0436\u0438\u0434\u0430\u043D\u0438\u044F \u0441\u043E\u043E\u0431\u0449\u0435\u043D\u0438\u044F \u043E\u0442 '${req.operation.id}'`));
        }
      }, timeoutMs);
      const parseFrame2 = (data) => {
        if (typeof data !== "string") return data;
        try {
          return JSON.parse(data);
        } catch {
          return data;
        }
      };
      ws.addEventListener("message", (event) => {
        if (settled) return;
        settled = true;
        const frame = event.data;
        cleanup();
        resolve({ status: 200, ok: true, body: parseFrame2(frame) });
      });
      ws.addEventListener("error", (event) => {
        if (settled) return;
        settled = true;
        cleanup();
        reject(new TransportError(`WebSocket-\u043E\u0448\u0438\u0431\u043A\u0430 \u0434\u043B\u044F '${req.operation.id}'`, { message: event?.message }));
      });
      ws.addEventListener("open", () => {
      });
      ws.addEventListener("close", (event) => {
        if (settled) return;
        settled = true;
        cleanup();
        reject(new TransportError(`WebSocket \u0437\u0430\u043A\u0440\u044B\u0442 \u0434\u043B\u044F '${req.operation.id}' (${event?.code ?? "unknown"})`));
      });
    });
  }
  close() {
    if (this.socket) {
      try {
        this.socket.close();
      } catch {
      }
      this.socket = null;
    }
  }
  /** Push-подписка: каждое сообщение → onData; возвращённая функция отписывает. */
  subscribe(req, onData, onError) {
    const url = wsSubscribeUrl(req.operation);
    if (!url) {
      const err = new TransportError(`\u041D\u0435\u0442 URL \u0434\u043B\u044F WebSocket-\u043A\u0430\u043D\u0430\u043B\u0430 \u043E\u043F\u0435\u0440\u0430\u0446\u0438\u0438 '${req.operation.id}'`);
      if (onError) onError(err);
      return () => void 0;
    }
    const ws = new this.ctor(url);
    this.socket = ws;
    const parseFrame2 = (data) => {
      if (typeof data !== "string") return data;
      try {
        return JSON.parse(data);
      } catch {
        return data;
      }
    };
    ws.addEventListener("message", (event) => {
      const frame = event.data;
      onData(parseFrame2(frame));
    });
    ws.addEventListener("error", (event) => {
      onError?.(new TransportError(`WebSocket-\u043E\u0448\u0438\u0431\u043A\u0430 \u0434\u043B\u044F '${req.operation.id}'`, { message: event?.message }));
    });
    ws.addEventListener("close", () => {
      onError?.(new TransportError(`WebSocket-\u043A\u0430\u043D\u0430\u043B \u0437\u0430\u043A\u0440\u044B\u0442 \u0434\u043B\u044F '${req.operation.id}'`));
    });
    let closed = false;
    return () => {
      if (closed) return;
      closed = true;
      try {
        ws.close();
      } catch {
      }
      if (this.socket === ws) this.socket = null;
    };
  }
};
var SseTransport = class {
  constructor(ctor) {
    this.ctor = ctor;
    this.source = null;
  }
  async request(req) {
    const url = wsSubscribeUrl(req.operation);
    if (!url) throw new SubscriptionError(`SSE-\u043A\u0430\u043D\u0430\u043B '${req.operation.id}' \u043D\u0435 \u0438\u043C\u0435\u0435\u0442 url`);
    const eventName = req.operation.subscribe?.eventName ?? req.operation.provider.subscribe?.eventName ?? "message";
    const timeoutMs = req.timeoutMs ?? 3e4;
    const body = await new Promise((resolve, reject) => {
      let settled = false;
      const source = new this.ctor(url);
      this.source = source;
      const cleanup = () => {
        try {
          source.close();
        } catch {
        }
        if (timer) clearTimeout(timer);
      };
      const timer = setTimeout(() => {
        if (!settled) {
          settled = true;
          cleanup();
          reject(new SubscriptionError(`\u0422\u0430\u0439\u043C\u0430\u0443\u0442 SSE-\u043F\u043E\u0434\u043F\u0438\u0441\u043A\u0438 '${req.operation.id}'`));
        }
      }, timeoutMs);
      const onMessage = (event) => {
        if (settled) return;
        settled = true;
        cleanup();
        const data = event.data;
        resolve(parseFrame(data));
      };
      const onError = () => {
        if (settled) return;
        settled = true;
        cleanup();
        reject(new SubscriptionError(`SSE-\u043E\u0448\u0438\u0431\u043A\u0430 \u0441\u043E\u0435\u0434\u0438\u043D\u0435\u043D\u0438\u044F '${req.operation.id}'`));
      };
      source.addEventListener(eventName, onMessage);
      source.addEventListener("error", onError);
    });
    return { status: 200, ok: true, body };
  }
  close() {
    if (this.source) {
      try {
        this.source.close();
      } catch {
      }
      this.source = null;
    }
  }
  /** Push-подписка: каждое событие → onData; возвращённая функция отписывает. */
  subscribe(req, onData, onError) {
    const url = wsSubscribeUrl(req.operation);
    if (!url) {
      onError?.(new SubscriptionError(`SSE-\u043A\u0430\u043D\u0430\u043B '${req.operation.id}' \u043D\u0435 \u0438\u043C\u0435\u0435\u0442 url`));
      return () => void 0;
    }
    const eventName = req.operation.subscribe?.eventName ?? req.operation.provider.subscribe?.eventName ?? "message";
    const source = new this.ctor(url);
    this.source = source;
    const onMessage = (event) => {
      onData(parseFrame(event.data));
    };
    const onErr = () => {
      onError?.(new SubscriptionError(`SSE-\u043E\u0448\u0438\u0431\u043A\u0430 \u0441\u043E\u0435\u0434\u0438\u043D\u0435\u043D\u0438\u044F '${req.operation.id}'`));
    };
    source.addEventListener(eventName, onMessage);
    source.addEventListener("error", onErr);
    let closed = false;
    return () => {
      if (closed) return;
      closed = true;
      try {
        source.close();
      } catch {
      }
      if (this.source === source) this.source = null;
    };
  }
};
function parseFrame(data) {
  if (typeof data !== "string") return data;
  try {
    return JSON.parse(data);
  } catch {
    return data;
  }
}

// ../../../ui-runtime/src/core/transport/transport.ts
function pickWebSocket(env) {
  const ctor = env?.WebSocket ?? globalThis.WebSocket;
  if (typeof ctor !== "function") {
    throw new Error("\u0422\u0440\u0430\u043D\u0441\u043F\u043E\u0440\u0442 \u0442\u0440\u0435\u0431\u0443\u0435\u0442 WebSocket");
  }
  return ctor;
}
function pickEventSource(env) {
  const ctor = env?.EventSource ?? globalThis.EventSource;
  if (typeof ctor !== "function") {
    throw new Error("\u0422\u0440\u0430\u043D\u0441\u043F\u043E\u0440\u0442 \u0442\u0440\u0435\u0431\u0443\u0435\u0442 EventSource");
  }
  return ctor;
}

// ../../../ui-runtime/src/core/transport/factory.ts
var TransportFactory = class {
  constructor(env) {
    this.env = env;
    this.cache = /* @__PURE__ */ new Map();
  }
  create(provider) {
    const cached = this.cache.get(provider.id);
    if (cached) return cached;
    let transport;
    switch (provider.protocol) {
      case "http":
        transport = new HttpTransport(provider, { fetch: this.env?.fetch });
        break;
      case "graphql":
        transport = new GraphqlTransport(provider, { fetch: this.env?.fetch });
        break;
      case "ws":
        transport = new WebSocketTransport(provider, pickWebSocket(this.env));
        break;
      case "sse":
        transport = new SseTransport(pickEventSource(this.env));
        break;
      default: {
        const exhaustive = provider.protocol;
        throw new Error(`\u041D\u0435\u0438\u0437\u0432\u0435\u0441\u0442\u043D\u044B\u0439 \u043F\u0440\u043E\u0442\u043E\u043A\u043E\u043B \u0442\u0440\u0430\u043D\u0441\u043F\u043E\u0440\u0442\u0430: ${String(exhaustive)}`);
      }
    }
    this.cache.set(provider.id, transport);
    return transport;
  }
  clear() {
    for (const t of this.cache.values()) t.close?.();
    this.cache.clear();
  }
};

// ../../../ui-runtime/src/core/tree.ts
function resolveDeclaration(declaration, ctx, resolveRuntime) {
  return {
    ...declaration,
    root: resolveInstance(declaration.root, ctx, resolveRuntime)
  };
}
function getPath(value, path) {
  const tokens = path.split(".").filter((t) => t !== "");
  let v = value;
  for (let i = 0; i < tokens.length; i++) {
    const t = tokens[i];
    if (t === "[]") {
      if (!Array.isArray(v)) return void 0;
      const rest = tokens.slice(i + 1).join(".");
      return rest ? v.map((item) => getPath(item, rest)) : v.slice();
    }
    if (v === null || typeof v !== "object") return void 0;
    const rec = v;
    v = Array.isArray(v) && /^\d+$/.test(t) ? v[Number(t)] : rec[t];
  }
  return v;
}
function operationValue(entry) {
  if (entry === null || typeof entry !== "object") return entry;
  const rec = entry;
  if (rec.error === true) return void 0;
  return rec.data !== void 0 ? rec.data : entry;
}
function resolveBinding(source, ctx, parentProps, resolveRuntime) {
  switch (source.type) {
    case "content":
      return getPath(ctx.content[source.contentId], source.path);
    case "routeParam":
      return ctx.route?.params[source.name];
    case "routeQuery":
      return ctx.route?.query[source.name];
    case "operation":
      return getPath(operationValue(ctx.operation[source.operationId]), source.path);
    case "form":
      return getPath(ctx.form[source.formId]?.values, source.path);
    case "props":
      return getPath(parentProps, source.path);
    case "runtime":
      return resolveRuntime?.(source.source);
  }
}
function resolveInstance(instance, ctx, resolveRuntime) {
  let props = { ...instance.props };
  for (const binding of instance.bindings) {
    const value = resolveBinding(binding.source, ctx, props, resolveRuntime);
    if (value !== void 0) {
      props = { ...props, [binding.property]: value };
    }
  }
  const children = instance.children.map(
    (child) => resolveInstance(child, ctx, resolveRuntime)
  );
  return {
    instanceId: instance.instanceId,
    definitionId: instance.definitionId,
    props,
    bindings: instance.bindings,
    children
  };
}
function findInstance(root, instanceId) {
  if (root.instanceId === instanceId) return root;
  for (const child of root.children) {
    const found = findInstance(child, instanceId);
    if (found) return found;
  }
  return null;
}
function cloneTree(root) {
  return {
    ...root,
    props: { ...root.props },
    children: root.children.map(cloneTree)
  };
}
var TreeController = class {
  constructor(store, opts) {
    this.store = store;
    this.listeners = /* @__PURE__ */ new Map();
    this.lastKey = "";
    this.resolveRuntime = opts?.resolveRuntime;
  }
  signature(d) {
    return JSON.stringify(d);
  }
  context() {
    const st = this.store.getState();
    return {
      content: st.content,
      route: st.route ? { path: st.route.route.id, params: st.route.params, query: st.route.query } : null,
      operation: st.operationResults,
      form: st.forms
    };
  }
  load(declaration) {
    this.lastKey = this.signature(declaration);
    this.store.getState().setTree(resolveDeclaration(declaration, this.context(), this.resolveRuntime));
    this.emit("update");
  }
  rebuild(next) {
    const key = this.signature(next);
    if (key === this.lastKey) return;
    this.lastKey = key;
    this.store.getState().setTree(resolveDeclaration(next, this.context(), this.resolveRuntime));
    this.emit("rebuild");
    this.emit("update");
  }
  get root() {
    const tree = this.store.getState().tree;
    return tree ? tree.root : null;
  }
  /** Резолвит декларацию с bindings в разрешённое дерево. */
  resolve(declaration, context) {
    const ctx = context ?? this.context();
    return resolveDeclaration(declaration, ctx, this.resolveRuntime);
  }
  /** Обновляет data-значения props по instanceId, не создавая onRebuild (§11#4). */
  updateBindings(patch) {
    const current = this.store.getState().tree;
    if (!current) return;
    const clone = cloneTree(current.root);
    for (const instanceId of Object.keys(patch)) {
      if (!findInstance(clone, instanceId)) {
        throw new UnknownEntityError(`\u0418\u043D\u0441\u0442\u0430\u043D\u0441 '${instanceId}' \u043D\u0435 \u043D\u0430\u0439\u0434\u0435\u043D \u0432 \u0434\u0435\u0440\u0435\u0432\u0435`);
      }
    }
    for (const [instanceId, values] of Object.entries(patch)) {
      const target = findInstance(clone, instanceId);
      if (target) {
        target.props = { ...target.props, ...values };
      }
    }
    this.store.getState().setTree({ ...current, root: clone });
    this.emit("update");
  }
  onRebuild(listener) {
    return this.on("rebuild", listener);
  }
  onUpdate(listener) {
    return this.on("update", listener);
  }
  on(event, listener) {
    const list = this.listeners.get(event) ?? [];
    list.push(listener);
    this.listeners.set(event, list);
    return () => {
      this.listeners.set(
        event,
        (this.listeners.get(event) ?? []).filter((l) => l !== listener)
      );
    };
  }
  emit(event) {
    for (const listener of this.listeners.get(event) ?? []) listener();
  }
};

// ../../../ui-runtime/src/core/boot.ts
var DEFAULT_BASE_URL = "http://localhost";
var CONTRACT_PATH = "/runtime/contract";
var DEV_CHANNEL = "dev";
var sessions = /* @__PURE__ */ new Map();
var BootSession = class {
  constructor(siteId, environment, opts) {
    this.registry = new RuntimeRegistry();
    this.store = createRuntimeStore();
    this.ready = false;
    this.unsubs = [];
    this.siteId = siteId;
    this.environment = environment;
    this.opts = opts;
  }
  get baseUrl() {
    return this.opts.baseUrl ?? DEFAULT_BASE_URL;
  }
  async start() {
    if (!this.factory) {
      this.factory = new TransportFactory({
        fetch: this.opts.env?.fetch,
        WebSocket: this.opts.env?.WebSocket
      });
      this.transport = new CompoundTransport(this.factory);
    }
    registerBuiltin(this.registry);
    const res = await this.fetchContract();
    const text = await res.text();
    const parsed = parseDescriptors(text);
    const rawTree = this.extractTree(text);
    for (const p of parsed.providers) {
      if (!this.registry.hasProvider(p.id)) this.registry.registerProvider(p);
    }
    for (const op of parsed.operations) {
      if (op.poll?.schedule) this.validateCron(op);
      if (!this.registry.hasOperation(op.id)) this.registry.registerOperation(op);
    }
    for (const ep of parsed.endpoints) {
      if (!this.registry.hasEndpoint(ep.id)) this.registry.registerEndpoint(ep);
    }
    this.i18n = new I18n(
      { defaultLocale: parsed.contract.locale, ...this.opts.i18n },
      this.store,
      { storage: this.opts.env?.storage, navigatorLanguage: this.opts.env?.navigatorLanguage }
    );
    this.tokens = new DesignTokens({ scope: ":root" });
    this.router = new Router(this.store, { window: this.opts.env?.window });
    this.sync = new SyncEngine(this.registry, this.transport);
    this.client = new ApiClient(this.registry, { transport: this.transport, scope: "server", sync: this.sync });
    this.tree = new TreeController(this.store);
    this.forms = new FormRuntime(this.store, {
      query: (operationId, input) => this.client.query(operationId, input),
      callEndpoint: (endpointId, input) => this.client.callEndpoint(endpointId, input)
    }, {
      localeProvider: () => this.i18n.getLocale()
    });
    this.assets = new AssetResolver(this.store, this.client.builtin.asset);
    this.i18n.detect();
    this.ready = true;
    if (parsed.themes.length > 0) {
      this.tokens.apply(parsed.themes[0]);
      this.store.getState().applyTokens(this.tokens.values());
    }
    this.router.replaceRoutes(parsed.routes);
    this.store.getState().setRoutes(parsed.routes);
    if (rawTree) this.tree.load(rawTree);
    this.startPolls(parsed.operations);
    this.startDevChannel(parsed.contract.capabilities.dev);
  }
  runtime() {
    return {
      siteId: this.siteId,
      environment: this.environment,
      ready: this.ready,
      registry: this.registry,
      store: this.store,
      router: this.router,
      sync: this.sync,
      client: this.client,
      i18n: this.i18n,
      tokens: this.tokens,
      tree: this.tree,
      forms: this.forms,
      assets: this.assets,
      dispose: () => this.dispose()
    };
  }
  reset(environment, opts) {
    this.environment = environment;
    this.opts = opts;
    this.registry.drop();
    this.sync?.dispose();
    this.closeDev();
    for (const unsub of this.unsubs) unsub();
    this.unsubs = [];
    this.ready = false;
  }
  dispose() {
    for (const unsub of this.unsubs) unsub();
    this.unsubs = [];
    this.closeDev();
    this.sync.dispose();
    this.factory.clear();
    sessions.delete(this.siteId);
  }
  closeDev() {
    this.dev?.close();
    this.dev = void 0;
  }
  async fetchContract() {
    const params = new URLSearchParams({ siteId: this.siteId, environment: this.environment });
    if (this.opts.versionId) params.set("versionId", this.opts.versionId);
    const fetchLike = this.opts.env?.fetch;
    if (typeof fetchLike !== "function") {
      throw new TransportError("boot \u0442\u0440\u0435\u0431\u0443\u0435\u0442 fetch (\u0433\u043B\u043E\u0431\u0430\u043B\u044C\u043D\u044B\u0439 \u0438\u043B\u0438 \u0438\u043D\u0436\u0435\u043A\u0442\u0438\u0440\u043E\u0432\u0430\u043D\u043D\u044B\u0439 \u0432 env.fetch)");
    }
    const res = await fetchLike(`${this.baseUrl}${CONTRACT_PATH}?${params.toString()}`);
    if (!res.ok) {
      throw new TransportError(`\u041A\u043E\u043D\u0442\u0440\u0430\u043A\u0442 \u0441\u0430\u0439\u0442\u0430 '${this.siteId}' \u043D\u0435\u0434\u043E\u0441\u0442\u0443\u043F\u0435\u043D: HTTP ${res.status}`, {
        status: res.status
      });
    }
    return res;
  }
  validateCron(op) {
    try {
      parseCron(op.poll.schedule);
    } catch {
      throw new DescriptorValidationError(
        `\u041D\u0435\u0432\u0430\u043B\u0438\u0434\u043D\u044B\u0439 poll.schedule '${op.poll.schedule}' \u043E\u043F\u0435\u0440\u0430\u0446\u0438\u0438 '${op.id}'`,
        { entityId: op.id, path: "poll.schedule" }
      );
    }
  }
  extractTree(text) {
    let raw;
    try {
      raw = JSON.parse(text);
    } catch {
      raw = {};
    }
    const tree = raw.tree;
    if (tree === null || typeof tree !== "object" || Array.isArray(tree)) return null;
    return tree;
  }
  startPolls(ops) {
    for (const op of ops) {
      const schedule = op.poll?.schedule;
      if (!schedule) continue;
      const opId = op.id;
      const push = this.sync.subscribe(
        opId,
        (data) => {
          this.store.getState().setOperationResult(opId, { data });
        },
        {
          input: { siteId: this.siteId, locale: this.i18n.getLocale() },
          params: { siteId: this.siteId, locale: this.i18n.getLocale() },
          errorHandler: this.opts.pollErrorHandler
        }
      );
      this.unsubs.push(push);
    }
  }
  startDevChannel(capDev) {
    if (this.environment !== "development" || !capDev) return;
    const WS = this.opts.env?.WebSocket;
    if (!WS) return;
    const ws = new WS(`${this.baseUrl}/runtime/dev`, DEV_CHANNEL);
    ws.addEventListener("message", (event) => {
      const data = event.data;
      if (!data) return;
      let msg;
      try {
        msg = JSON.parse(data);
      } catch {
        return;
      }
      if (msg.type === "tree" && msg.tree) this.tree.rebuild(msg.tree);
    });
    this.dev = ws;
  }
};
async function boot(siteId, environment = "production", opts = {}) {
  const existing = sessions.get(siteId);
  if (existing) {
    existing.reset(environment, opts);
  }
  const session = existing ?? new BootSession(siteId, environment, opts);
  await session.start();
  sessions.set(siteId, session);
  return session.runtime();
}
export {
  boot
};
