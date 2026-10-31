import { expect, type Page } from "@playwright/test";
export async function completeOnboarding(
  page: Page,
  imperial = false,
  manual = false,
) {
  await expect(
    page.getByRole("heading", { name: "Let’s start with you." }),
  ).toBeVisible();
  if (imperial)
    await page
      .getByLabel("Preferred measurement units")
      .selectOption("imperial");
  await page.getByLabel("Age in years", { exact: true }).fill("24");
  await page
    .getByLabel("Sex coefficient for metabolic estimation")
    .selectOption(manual ? "unspecified" : "male");
  if (imperial) {
    await page.getByLabel("Height (feet)", { exact: true }).fill("5");
    await page.getByLabel("Height (inches)", { exact: true }).fill("6");
    await page
      .getByLabel("Current weight (lb)", { exact: true })
      .fill(String(54.5 / 0.45359237));
  } else {
    await page.getByLabel("Height (cm)", { exact: true }).fill("167.64");
    await page.getByLabel("Current weight (kg)", { exact: true }).fill("54.5");
  }
  await page.getByRole("button", { name: "Next", exact: true }).click();
  await page.getByLabel("Everyday lifestyle").selectOption("sedentary");
  await page.getByLabel("Workout sessions per week").fill("3.5");
  await page.getByLabel("Average minutes per session").fill("60");
  await page.getByLabel("Workout intensity").selectOption("moderate");
  await page.getByLabel("Primary training type").selectOption("resistance");
  await page.getByRole("button", { name: "Next", exact: true }).click();
  await page.getByLabel("Fitness objective").selectOption("muscle");
  await page.getByRole("button", { name: "Next", exact: true }).click();
  await page.getByLabel("Dietary preference").selectOption("vegetarian");
  await page
    .getByLabel("Health and suitability")
    .selectOption(manual ? "clinician" : "healthy");
  await page.getByRole("button", { name: "Next", exact: true }).click();
  await expect(
    page.getByRole("heading", { name: "A quick check before we calculate." }),
  ).toBeVisible();
  // Check that Previous returns to editable fields without losing the form.
  await page.getByRole("button", { name: "Previous", exact: true }).click();
  await expect(page.getByLabel("Dietary preference")).toHaveValue("vegetarian");
  await page.getByRole("button", { name: "Next", exact: true }).click();
  await page.getByRole("button", { name: "Save profile & calculate" }).click();
  await expect(
    page.getByRole("heading", { name: "Your starting point, made personal." }),
  ).toBeVisible();
  const s = await page.evaluate(
    async () => await (await fetch("/api/onboarding")).json(),
  );
  expect(s.user.onboarding_completed).toBe(false);
  expect(s.user.goals.calories).toBe(0);
  if (!manual) {
    expect(s.recommendation.result.ree).toBeCloseTo(1477.75, 2);
    expect(s.recommendation.result.goals.protein).toBeCloseTo(98.1, 1);
    expect(s.recommendation.result.supported).toBe(true);
  } else expect(s.recommendation.result.supported).toBe(false);
  await page.screenshot({
    path: `test-results/onboarding-${imperial ? "mobile" : "desktop"}-${manual ? "manual" : "recommended"}.png`,
    fullPage: true,
  });
  // A saved result resumes after reload, before any goals are activated.
  await page.reload();
  await expect(
    page.getByRole("heading", { name: "Your starting point, made personal." }),
  ).toBeVisible();
  if (manual) {
    await page.getByLabel("Calories (kcal)", { exact: true }).fill("2100");
    await page.getByLabel("Protein (g)", { exact: true }).fill("100");
    await page.getByLabel("Carbohydrates (g)", { exact: true }).fill("260");
    await page.getByLabel("Fat (g)", { exact: true }).fill("70");
    await page.getByLabel("Fibre (g)", { exact: true }).fill("30");
    await page
      .getByRole("button", { name: "Save custom goals", exact: true })
      .click();
  } else
    await page
      .getByRole("button", { name: "Accept recommended goals", exact: true })
      .click();
  await expect(
    page.getByRole("heading", { name: "Looking good, Journey." }),
  ).toBeVisible();
}
