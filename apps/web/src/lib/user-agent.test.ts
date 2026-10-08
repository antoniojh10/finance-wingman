import { describe, expect, it } from "vitest";

import { describeUserAgent } from "./user-agent";

describe("describeUserAgent", () => {
  it.each([
    [
      "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/129.0.0.0 Safari/537.36",
      { browser: "Chrome", os: "macOS", mobile: false },
    ],
    [
      "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/129.0.0.0 Safari/537.36 Edg/129.0.0.0",
      { browser: "Edge", os: "Windows", mobile: false },
    ],
    ["Mozilla/5.0 (X11; Linux x86_64; rv:131.0) Gecko/20100101 Firefox/131.0", { browser: "Firefox", os: "Linux", mobile: false }],
    [
      "Mozilla/5.0 (iPhone; CPU iPhone OS 18_0 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/18.0 Mobile/15E148 Safari/604.1",
      { browser: "Safari", os: "iOS", mobile: true },
    ],
    [
      "Mozilla/5.0 (Linux; Android 14; Pixel 7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/129.0.0.0 Mobile Safari/537.36",
      { browser: "Chrome", os: "Android", mobile: true },
    ],
    [
      "Mozilla/5.0 (Linux; Android 14; SM-S918B) AppleWebKit/537.36 (KHTML, like Gecko) SamsungBrowser/26.0 Chrome/122.0.0.0 Mobile Safari/537.36",
      { browser: "Samsung Internet", os: "Android", mobile: true },
    ],
  ])("recognizes %s", (userAgent, expected) => {
    expect(describeUserAgent(userAgent)).toEqual(expected);
  });

  it("leaves unknown parts out", () => {
    expect(describeUserAgent("curl/8.0")).toEqual({ browser: undefined, os: undefined, mobile: false });
    expect(describeUserAgent("")).toEqual({ browser: undefined, os: undefined, mobile: false });
  });
});
