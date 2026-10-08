import { expect, test as base } from "@playwright/test";

/**
 * `test` with a guard on every page: the test fails if the browser reports a
 * Content-Security-Policy violation.
 */
export const test = base.extend<{ cspGuard: void }>({
  cspGuard: [
    async ({ page }, use) => {
      const violations: string[] = [];
      await page.exposeFunction("__reportCspViolation", (message: string) => violations.push(message));
      await page.addInitScript(() => {
        document.addEventListener("securitypolicyviolation", (event) => {
          const report = (window as unknown as { __reportCspViolation: (message: string) => void }).__reportCspViolation;
          report(`${event.violatedDirective} blocked ${event.blockedURI || "inline"} (${event.sourceFile}:${event.lineNumber})`);
        });
      });
      await use();
      expect(violations, "Content-Security-Policy violations").toEqual([]);
    },
    { auto: true },
  ],
});

export { expect };
