import http from "k6/http";
import { check, sleep } from "k6";

const BASE_URL = __ENV.BASE_URL || "http://localhost:8080";
const EVENT_ID = __ENV.EVENT_ID; // optional for /register test
const TOKEN = __ENV.TOKEN;       // optional for /register test

export const options = {
  vus: 1,
  duration: "10s",
};

export default function () {
  // 1) health
  const h = http.get(`${BASE_URL}/healthz`);
  check(h, { "healthz 200": (r) => r.status === 200 });

  // 2) list events
  const e = http.get(`${BASE_URL}/events`);
  check(e, { "events 200": (r) => r.status === 200 });

  // 3) optional: register
  if (EVENT_ID && TOKEN) {
    const r = http.post(
      `${BASE_URL}/events/${EVENT_ID}/register`,
      JSON.stringify({ name: "K6 Smoke", email: `smoke-${__VU}-${__ITER}@test.dev` }),
      {
        headers: {
          "Content-Type": "application/json",
          Authorization: `Bearer ${TOKEN}`,
        },
      }
    );

    // register might return 201/200; if duplicates can return 409; still “working”
    check(r, {
      "register ok (2xx/409)": (x) => (x.status >= 200 && x.status < 300) || x.status === 409,
    });
  }

  sleep(1);
}
