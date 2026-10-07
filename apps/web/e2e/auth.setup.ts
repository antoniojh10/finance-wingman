import { test as setup } from "@playwright/test";

import { authFile, e2eUser } from "../playwright.config";
import { signIn } from "./helpers";

// Signs in once and saves the session for the tests that start signed in.
setup("sign in", async ({ page }) => {
  await signIn(page, e2eUser);
  await page.context().storageState({ path: authFile });
});
