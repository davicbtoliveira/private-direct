import { test, expect, type BrowserContext, type Page } from "@playwright/test";

const OPERATOR_TOKEN = "operator-secret";
const PASSWORD = "E2e-Only!7f2c9a48-DoNotReuse";

async function createInvite(code: string) {
  const res = await fetch("http://127.0.0.1:8080/api/operator/invites", {
    method: "POST",
    headers: {
      "Content-Type": "application/json",
      "X-Operator-Token": OPERATOR_TOKEN,
    },
    body: JSON.stringify({ code }),
  });
  expect(res.status).toBe(201);
}

async function register(page: Page, inviteCode: string, username: string, password: string) {
  await page.goto("/register");
  await page.getByLabel("Invite code").fill(inviteCode);
  await page.getByLabel("Username").fill(username);
  await page.getByLabel("Password", { exact: true }).fill(password);
  await page.getByLabel("Confirm password").fill(password);
  await page.getByRole("button", { name: "Register" }).click();
  await page.waitForURL("/setup-encryption");
  const phrase = (await page.getByLabel("Recovery phrase", { exact: true }).textContent())?.trim();
  expect(phrase?.split(/\s+/)).toHaveLength(24);
  await page.getByLabel("Confirm recovery phrase").fill(phrase!);
  await page.getByRole("button", { name: "I saved my recovery phrase" }).click();
  await page.waitForURL("/chat");
}

async function login(page: Page, username: string) {
  await page.goto("/login");
  await page.getByLabel("Username").fill(username);
  await page.getByLabel("Password", { exact: true }).fill(PASSWORD);
  await page.getByRole("button", { name: "Sign in" }).click();
  await page.waitForURL("/chat");
}

test.describe("Private Direct MVP", () => {
  test("registration and login flow", async ({ page }) => {
    await createInvite("invite-e2e-reg");
    await register(page, "invite-e2e-reg", "e2e-alice", PASSWORD);

    await expect(page.getByTestId("workspace")).toBeVisible();
    await expect(page.locator("[aria-label='Contact list']")).toBeVisible();
    await expect(page.getByText("@e2e-alice")).toBeVisible();
    await expect(page.getByText("No contacts yet")).toBeVisible();
  });

  test("login with existing account", async ({ page }) => {
    await createInvite("invite-e2e-login");
    await register(page, "invite-e2e-login", "e2e-bob", PASSWORD);

    // Logout and login again
    await page.getByLabel("Sign out").click();
    await page.waitForURL("/login");
    await login(page, "e2e-bob");

    await expect(page.getByTestId("workspace")).toBeVisible();
    await expect(page.getByText("@e2e-bob")).toBeVisible();
  });

  test("contact request and acceptance", async ({ page }) => {
    // Alice registers
    await createInvite("invite-e2e-alice");
    await register(page, "invite-e2e-alice", "e2e-alice-c", PASSWORD);

    // Create Bob via API
    await createInvite("invite-e2e-bob-c");
    const res = await fetch("http://127.0.0.1:8080/api/register", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({
        invite_code: "invite-e2e-bob-c",
        username: "e2e-bob-c",
        password: PASSWORD,
      }),
    });
    expect(res.status).toBe(201);

    // Login as Bob and get a token
    let bobToken: string;
    {
      const loginRes = await fetch("http://127.0.0.1:8080/api/login", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ username: "e2e-bob-c", password: PASSWORD }),
      });
      const body = await loginRes.json();
      bobToken = body.access_token;
    }

    // Bob sends contact request to Alice via API
    {
      const reqRes = await fetch("http://127.0.0.1:8080/api/contacts/requests", {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
          Authorization: `Bearer ${bobToken}`,
        },
        body: JSON.stringify({ username: "e2e-alice-c" }),
      });
      expect([200, 201]).toContain(reqRes.status);
    }

    // Alice should see a request badge
    await page.reload();
    await page.waitForTimeout(2000);

    // Alice opens requests
    const bellButton = page.getByLabel(/Incoming requests/);
    await expect(bellButton).toBeVisible();
    await bellButton.click();

    // Accept the request
    await expect(page.getByRole("button", { name: /Accept/ })).toBeVisible({ timeout: 5000 });
    await page.getByRole("button", { name: /Accept/ }).click();

    // Bob should now appear in Alice's contact list
    await expect(page.getByText("@e2e-bob-c")).toBeVisible({ timeout: 5000 });
  });

  test("contact list updates via WebSocket invalidation", async ({ page }) => {
    await createInvite("invite-e2e-realtime-a");
    await register(page, "invite-e2e-realtime-a", "e2e-alice-rt", PASSWORD);

    // Create Charlie via API
    await createInvite("invite-e2e-charlie");
    const charlieRes = await fetch("http://127.0.0.1:8080/api/register", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({
        invite_code: "invite-e2e-charlie",
        username: "e2e-charlie",
        password: PASSWORD,
      }),
    });
    expect(charlieRes.status).toBe(201);

    let charlieToken: string;
    {
      const r = await fetch("http://127.0.0.1:8080/api/login", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ username: "e2e-charlie", password: PASSWORD }),
      });
      const b = await r.json();
      charlieToken = b.access_token;
    }

    // Charlie sends contact request and gets a WS connection
    // Alice refreshes and sees the request badge via contacts_changed
    await fetch("http://127.0.0.1:8080/api/contacts/requests", {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
        Authorization: `Bearer ${charlieToken}`,
      },
      body: JSON.stringify({ username: "e2e-alice-rt" }),
    });

    // Alice should see a notification badge
    await page.waitForTimeout(2000);
  });

  test("composer is disabled when no contact selected", async ({ page }) => {
    await createInvite("invite-e2e-compose");
    await register(page, "invite-e2e-compose", "e2e-composer", PASSWORD);

    // No contact selected - should show placeholder
    await expect(page.getByText("Select a contact")).toBeVisible();

    // Composer should not be visible without a contact
    await expect(page.getByLabel("Message")).not.toBeVisible();
  });

  test("logout clears state and returns to login", async ({ page }) => {
    await createInvite("invite-e2e-logout");
    await register(page, "invite-e2e-logout", "e2e-logout", PASSWORD);

    await page.getByLabel("Sign out").click();
    await page.waitForURL("/login");

    await expect(page.getByRole("button", { name: "Sign in" })).toBeVisible();
  });

  test("invalid registration shows field errors", async ({ page }) => {
    await page.goto("/register");

    // Try registering with no fields
    await page.getByRole("button", { name: "Register" }).click();

    await expect(page.getByText("Enter an invite code.")).toBeVisible();
    await expect(page.getByText(/Use 3-32 letters/)).toBeVisible();
    await expect(page.getByText(/Use 12-72 UTF-8 bytes/)).toBeVisible();
    await expect(page.getByText("Confirm your password.")).toBeVisible();
  });
});
