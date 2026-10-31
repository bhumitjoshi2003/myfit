import { completeOnboarding } from "./onboarding-helper";
import { test, expect } from "@playwright/test";
test("nutrition diary, weight, private food, goals and persistent login", async ({
  page,
}, info) => {
  const email = `myfit-e2e-${Date.now()}-${info.project.name}@example.test`,
    password = "myfit-test-password-123";
  const errors: string[] = [];
  page.on("pageerror", (e) => errors.push(e.message));
  await page.goto("/");
  await page
    .getByRole("button", { name: "Create an account", exact: true })
    .click();
  await page.getByLabel("Your name").fill("Journey");
  await page.getByLabel("Email address").fill(email);
  await page.getByLabel("Password", { exact: true }).fill(password);
  await page
    .getByRole("button", { name: "Create account", exact: true })
    .click();
  await completeOnboarding(page, info.project.name === "mobile");
  await expect(
    page.getByRole("heading", { name: "Looking good, Journey." }),
  ).toBeVisible();
  await page.getByRole("button", { name: "Log food", exact: true }).click();
  await page.getByRole("textbox", { name: "Search foods" }).fill("Banana");
  await page.getByRole("button", { name: /Banana, raw Shared/ }).click();
  await page
    .getByLabel("Serving size")
    .selectOption({ label: "Medium banana (edible portion) · 118 g" });
  await page.getByLabel("Quantity", { exact: true }).fill("2");
  await page.getByRole("button", { name: "Add to breakfast" }).click();
  await expect(page.locator(".calorie-ring strong")).toHaveText("210");
  const assertTotal = async (cal: number) => {
    const data = await page.evaluate(
      async () => await (await fetch("/api/diary")).json(),
    );
    expect(data.totals.calories).toBeCloseTo(cal, 3);
    return data;
  };
  const first = await assertTotal(210.04);
  expect(first.totals.protein).toBeCloseTo(2.5724, 4);
  expect(first.totals.carbs).toBeCloseTo(53.9024, 4);
  expect(first.totals.fat).toBeCloseTo(0.7788, 4);
  expect(first.totals.fibre).toBeCloseTo(6.136, 3);
  const nav = async (name: string) => {
    if (info.project.name === "mobile")
      await page.getByRole("button", { name: "Toggle menu" }).click();
    await page.getByRole("button", { name, exact: true }).click();
  };
  await nav("Food Diary");
  await page
    .getByRole("button", { name: "Edit Banana, raw", exact: true })
    .click();
  await page.getByLabel("Quantity", { exact: true }).fill("1");
  await page.getByRole("button", { name: "Save changes", exact: true }).click();
  await expect(page.locator(".entry-description").first()).toContainText(
    "1 × Medium banana",
  );
  await assertTotal(105.02);
  await nav("Weight Progress");
  await page
    .getByRole("button", { name: "Record weight", exact: true })
    .first()
    .click();
  await page.getByLabel("Body weight (kg)", { exact: true }).fill("54.5");
  await page.getByRole("button", { name: "Save weight", exact: true }).click();
  await expect(page.locator(".weight-stats")).toContainText("54.5 kg");
  await expect(page.getByRole("img", { name: /Weight trend/ })).toBeVisible();
  await nav("Food Library");
  await page.getByRole("button", { name: "Create food", exact: true }).click();
  await page.getByLabel("Food name", { exact: true }).fill("My label oats");
  await page.getByLabel("Calories (kcal)", { exact: true }).fill("375");
  await page.getByLabel("Protein (g)", { exact: true }).fill("12");
  await page
    .getByRole("button", { name: "Save private food", exact: true })
    .click();
  await page
    .getByRole("textbox", { name: "Search library" })
    .fill("My label oats");
  await expect(page.locator(".library-row")).toContainText("Private");
  await page
    .getByRole("button", { name: "Favourite My label oats", exact: true })
    .click();
  await expect(
    page.getByRole("button", {
      name: "Unfavourite My label oats",
      exact: true,
    }),
  ).toBeVisible();
  await nav("Profile / Goals");
  await page.getByLabel("Calories (kcal)", { exact: true }).fill("2100");
  await page.getByRole("button", { name: "Save profile & goals" }).click();
  await expect(page.getByRole("status")).toContainText("saved");
  await page
    .getByRole("button", { name: "Update profile & review recommendations" })
    .click();
  await page
    .getByLabel(
      info.project.name === "mobile"
        ? "Current weight (lb)"
        : "Current weight (kg)",
      { exact: true },
    )
    .fill(String(info.project.name === "mobile" ? 57 / 0.45359237 : 57));
  for (let i = 0; i < 4; i++)
    await page.getByRole("button", { name: "Next", exact: true }).click();
  await page.getByRole("button", { name: "Save profile & calculate" }).click();
  await expect(
    page.getByRole("heading", { name: "Review the change" }),
  ).toBeVisible();
  const pending = await page.evaluate(
    async () => await (await fetch("/api/onboarding")).json(),
  );
  expect(pending.user.goals.calories).toBe(2100);
  expect(pending.user.goals_source).toBe("custom");
  await page
    .getByRole("button", { name: "Keep current goals", exact: true })
    .click();
  if (info.project.name === "mobile")
    await page.getByRole("button", { name: "Toggle menu" }).click();
  await page.getByRole("button", { name: "Sign out", exact: true }).click();
  await expect(
    page.getByRole("heading", { name: "Good to see you again." }),
  ).toBeVisible();
  await page.getByLabel("Email address").fill(email);
  await page.getByLabel("Password", { exact: true }).fill(password);
  await page.getByRole("button", { name: "Sign in", exact: true }).click();
  await expect(page.locator(".calorie-ring strong")).toHaveText("105");
  await expect(page.locator(".calorie-ring")).toContainText("2,100");
  await assertTotal(105.02);
  await expect(page.locator(".weight-summary")).toContainText("54.5");
  if (info.project.name === "mobile")
    await expect(page.locator(".sidebar")).toBeHidden();
  expect(
    await page.evaluate(
      () => document.documentElement.scrollWidth <= window.innerWidth,
    ),
  ).toBe(true);
  expect(errors).toEqual([]);
  await page.screenshot({
    path: `test-results/${info.project.name}-dashboard.png`,
    fullPage: true,
  });
  const cookie = (await page.context().cookies()).find(
    (c) => c.name === "myfit_session",
  );
  expect(cookie?.httpOnly).toBe(true);
  expect(cookie?.sameSite).toBe("Lax");
  expect(await page.evaluate(() => localStorage.length)).toBe(0);
});
