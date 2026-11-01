// Calendar-day helpers. A "day" is always a YYYY-MM-DD string in the user's
// own time zone; it is never derived from UTC (toISOString) or the server.

export function today(tz: string, now: Date = new Date()): string {
  try {
    return new Intl.DateTimeFormat("en-CA", {
      timeZone: tz,
      year: "numeric",
      month: "2-digit",
      day: "2-digit",
    }).format(now);
  } catch {
    // Unknown zone: fall back to the device's own calendar day.
    return new Intl.DateTimeFormat("en-CA", {
      year: "numeric",
      month: "2-digit",
      day: "2-digit",
    }).format(now);
  }
}

export function shiftDay(day: string, delta: number): string {
  const d = new Date(day + "T12:00:00Z");
  d.setUTCDate(d.getUTCDate() + delta);
  return d.toISOString().slice(0, 10);
}

// When the calendar rolls over, a diary that was following "today" moves to
// the new day; a diary the user deliberately pointed at another date stays put.
export function rollover(
  selected: string,
  previousToday: string,
  currentToday: string,
): string {
  return selected === previousToday ? currentToday : selected;
}
