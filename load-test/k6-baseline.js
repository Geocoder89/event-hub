import http from "k6/http";
import { check, sleep } from "k6";

const BASE_URL = __ENV.BASE_URL || "http://localhost:8080";

// seed known event IDs (pass as ENV comma-separated)
const EVENT_IDS = (__ENV.EVENT_IDS || "")
  .split(",")
  .map((s) => s.trim())
  .filter(Boolean);

// events that are known to have registrations (seed these) 
const REG_EVENT_IDS = (__ENV.REG_EVENT_IDS || "")
  .split(",")
  .map((s) => s.trim())
  .filter(Boolean);

// admin token for /admin/jobs (and also for authed registrations routes)
const ADMIN_TOKEN = __ENV.ADMIN_TOKEN || "";

// optional jobs filter: "failed" | "pending" | "processing" | "done"
const JOB_STATUS = __ENV.JOB_STATUS || "";

const LIST_LIMIT = Number(__ENV.LIST_LIMIT || 20);

export const options = {
  scenarios: {
    warmup: {
      executor: "constant-vus",
      vus: Number(__ENV.WARMUP_VUS || 5),
      duration: __ENV.WARMUP || "30s",
      gracefulStop: "5s",
    },
    baseline: {
      executor: "constant-arrival-rate",
      startTime: __ENV.WARMUP || "30s",
      rate: Number(__ENV.RATE || 50),
      timeUnit: "1s",
      duration: __ENV.DURATION || "2m",
      preAllocatedVUs: Number(__ENV.PRE_VUS || 50),
      maxVUs: Number(__ENV.MAX_VUS || 200),
    },
  },
  thresholds: {
    // EVENTS list first page (no cursor)
    "http_req_failed{op:read_list_first}": ["rate<0.01"],
    "http_req_duration{op:read_list_first}": ["p(95)<500"],

    // EVENTS list next page (cursor)
    "http_req_failed{op:read_list_next}": ["rate<0.01"],
    "http_req_duration{op:read_list_next}": ["p(95)<500"],

    // EVENTS detail
    "http_req_failed{op:read_detail}": ["rate<0.01"],
    "http_req_duration{op:read_detail}": ["p(95)<500"],

    // REGISTRATIONS list first page (no cursor)
    "http_req_failed{op:read_regs_first}": ["rate<0.01"],
    "http_req_duration{op:read_regs_first}": ["p(95)<500"],

    // REGISTRATIONS list next page (cursor)
    "http_req_failed{op:read_regs_next}": ["rate<0.01"],
    "http_req_duration{op:read_regs_next}": ["p(95)<500"],

    // ADMIN JOBS list first page (no cursor)
    "http_req_failed{op:read_jobs_first}": ["rate<0.01"],
    "http_req_duration{op:read_jobs_first}": ["p(95)<500"],

    // ADMIN JOBS list next page (cursor)
    "http_req_failed{op:read_jobs_next}": ["rate<0.01"],
    "http_req_duration{op:read_jobs_next}": ["p(95)<500"],
  },
};

function tryJson(res) {
  try {
    return res.json();
  } catch (_) {
    return null;
  }
}

function pick(arr) {
  return arr[Math.floor(Math.random() * arr.length)];
}

// Print the first failing registrations response once (helps debugging 401/404/400)
let printedRegsError = false;
function debugRegsOnce(res, url) {
  if (!printedRegsError && res.status >= 400) {
    printedRegsError = true;
    console.log(`REGS_FIRST FAILED status=${res.status} url=${url} body=${res.body}`);
  }
}

// -------------- EVENTS --------------
function runEventsFlow() {
  const res1 = http.get(`${BASE_URL}/events?limit=${LIST_LIMIT}`, {
    tags: { op: "read_list_first", endpoint: "GET /events" },
  });

  check(res1, { "GET /events (first) 200": (r) => r.status === 200 });

  const body1 = tryJson(res1);
  const cursor = body1 && body1.nextCursor ? body1.nextCursor : null;

  if (cursor) {
    const res2 = http.get(
      `${BASE_URL}/events?limit=${LIST_LIMIT}&cursor=${encodeURIComponent(cursor)}`,
      { tags: { op: "read_list_next", endpoint: "GET /events?cursor" } }
    );

    check(res2, {
      "GET /events (next) 200": (r) => r.status === 200,
      "events next page has items[]": (r) => Array.isArray(r.json("items")),
    });
  }
}

// -------------- REGISTRATIONS --------------
// Assumes this route is protected; uses ADMIN_TOKEN for now.
function runRegistrationsFlow() {
  if (REG_EVENT_IDS.length === 0) return;
  if (!ADMIN_TOKEN) return;

  const eventId = pick(REG_EVENT_IDS);

  const url1 = `${BASE_URL}/events/${eventId}/registrations?limit=${LIST_LIMIT}`;
  const res1 = http.get(url1, {
    headers: { Authorization: `Bearer ${ADMIN_TOKEN}` },
    tags: { op: "read_regs_first", endpoint: "GET /events/:id/registrations" },
  });

  debugRegsOnce(res1, url1);

  check(res1, {
    "GET registrations (first) 200": (r) => r.status === 200,
    "registrations first has items[]": (r) => Array.isArray(r.json("items")),
  });

  const body1 = tryJson(res1);
  const cursor = body1 && body1.nextCursor ? body1.nextCursor : null;

  if (cursor) {
    const url2 = `${BASE_URL}/events/${eventId}/registrations?limit=${LIST_LIMIT}&cursor=${encodeURIComponent(
      cursor
    )}`;
    const res2 = http.get(url2, {
      headers: { Authorization: `Bearer ${ADMIN_TOKEN}` },
      tags: { op: "read_regs_next", endpoint: "GET /events/:id/registrations?cursor" },
    });

    check(res2, {
      "GET registrations (next) 200": (r) => r.status === 200,
      "registrations next has items[]": (r) => Array.isArray(r.json("items")),
    });
  }
}

// -------------- ADMIN JOBS --------------
function runJobsFlow() {
  if (!ADMIN_TOKEN) return;

  const statusQ = JOB_STATUS ? `&status=${encodeURIComponent(JOB_STATUS)}` : "";

  const res1 = http.get(`${BASE_URL}/admin/jobs?limit=${LIST_LIMIT}${statusQ}`, {
    headers: { Authorization: `Bearer ${ADMIN_TOKEN}` },
    tags: { op: "read_jobs_first", endpoint: "GET /admin/jobs" },
  });

  check(res1, {
    "GET jobs (first) 200": (r) => r.status === 200,
    "jobs first has items[]": (r) => Array.isArray(r.json("items")),
  });

  const body1 = tryJson(res1);
  const cursor = body1 && body1.nextCursor ? body1.nextCursor : null;

  if (cursor) {
    const res2 = http.get(
      `${BASE_URL}/admin/jobs?limit=${LIST_LIMIT}${statusQ}&cursor=${encodeURIComponent(cursor)}`,
      {
        headers: { Authorization: `Bearer ${ADMIN_TOKEN}` },
        tags: { op: "read_jobs_next", endpoint: "GET /admin/jobs?cursor" },
      }
    );

    check(res2, {
      "GET jobs (next) 200": (r) => r.status === 200,
      "jobs next has items[]": (r) => Array.isArray(r.json("items")),
    });
  }
}

export default function () {
  const x = Math.random();

  // Traffic mix:
  // 55% events listing, 15% events detail, 15% registrations, 15% admin jobs
  if (x < 0.55) {
    runEventsFlow();
    sleep(0.1);
    return;
  }

  if (x < 0.70) {
    if (EVENT_IDS.length === 0) {
      runEventsFlow();
    } else {
      const id = pick(EVENT_IDS);
      const res = http.get(`${BASE_URL}/events/${id}`, {
        tags: { op: "read_detail", endpoint: "GET /events/:id" },
      });
      check(res, { "GET /events/:id 200": (r) => r.status === 200 });
    }
    sleep(0.1);
    return;
  }

  if (x < 0.85) {
    runRegistrationsFlow();
    sleep(0.1);
    return;
  }

  runJobsFlow();
  sleep(0.1);
}
