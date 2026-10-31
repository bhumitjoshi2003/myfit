import { test, expect } from "@playwright/test";
import { completeOnboarding } from "./onboarding-helper";
test("manual setup for unsupported estimation input", async ({
  page,
}, info) => {
  await page.goto("/");
  await page
    .getByRole("button", { name: "Create an account", exact: true })
    .click();
  await page.getByLabel("Your name").fill("Journey");
  await page
    .getByLabel("Email address")
    .fill(`myfit-e2e-manual-${Date.now()}-${info.project.name}@example.test`);
  await page
    .getByLabel("Password", { exact: true })
    .fill("manual-test-password-123");
  await page
    .getByRole("button", { name: "Create account", exact: true })
    .click();
  await completeOnboarding(page, info.project.name === "mobile", true);
  const u = await page.evaluate(
    async () => await (await fetch("/api/me")).json(),
  );
  expect(u.user.goals_source).toBe("custom");
  expect(u.user.goals.calories).toBe(2100);
  expect(
    await page.evaluate(
      () => document.documentElement.scrollWidth <= innerWidth,
    ),
  ).toBe(true);
});
