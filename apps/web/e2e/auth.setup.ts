import { test as setup } from "@playwright/test";

import { authFile, e2eUser } from "../playwright.config";
import { mintSession } from "./helpers";

// Opens a session straight in the database (no email flow, so no Mailpit) and
// saves it for the tests that start signed in. Sign-in by email is covered by
// auth.spec.ts.
setup("sign in", async ({ context }) => {
  await context.addCookies([mintSession(e2eUser)]);
  await context.storageState({ path: authFile });
});
