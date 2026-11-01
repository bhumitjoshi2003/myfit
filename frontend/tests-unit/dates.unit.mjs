import test from "node:test";
import assert from "node:assert/strict";
import { today, shiftDay, rollover } from "../src/dates.ts";

test("today follows the user's zone, not UTC", () => {
  const nineThirtyPmIst = new Date("2026-10-31T16:00:00Z");
  const midnightIst = new Date("2026-10-31T18:30:00Z");
  assert.equal(today("Asia/Kolkata", nineThirtyPmIst), "2026-10-31");
  assert.equal(today("Asia/Kolkata", midnightIst), "2026-11-01");
  assert.equal(today("UTC", midnightIst), "2026-10-31");
});

test("invalid zone falls back instead of throwing", () => {
  assert.match(today("Bad/Zone", new Date()), /^\d{4}-\d{2}-\d{2}$/);
});

test("shiftDay is calendar arithmetic across month ends", () => {
  assert.equal(shiftDay("2026-10-31", 1), "2026-11-01");
  assert.equal(shiftDay("2026-11-01", -1), "2026-10-31");
});

test("rollover moves a diary following today, respects a manual date", () => {
  assert.equal(rollover("2026-10-31", "2026-10-31", "2026-11-01"), "2026-11-01");
  assert.equal(rollover("2026-10-20", "2026-10-31", "2026-11-01"), "2026-10-20");
});
