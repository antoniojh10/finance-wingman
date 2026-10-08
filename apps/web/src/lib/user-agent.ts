/** The browser and operating system named by a User-Agent string, when recognized. */
export type DeviceDescription = { browser?: string; os?: string; mobile: boolean };

// Order matters: Edge, Opera and Samsung Internet also claim to be Chrome,
// and Chrome claims to be Safari.
const browsers: [RegExp, string][] = [
  [/\bEdg(e|A|iOS)?\//, "Edge"],
  [/\b(OPR|Opera)\//, "Opera"],
  [/\bSamsungBrowser\//, "Samsung Internet"],
  [/\b(Firefox|FxiOS)\//, "Firefox"],
  [/\b(Chrome|CriOS|Chromium)\//, "Chrome"],
  [/\bVersion\/[\d.]+.*\bSafari\//, "Safari"],
];

const systems: [RegExp, string][] = [
  [/\b(iPhone|iPad|iPod)\b/, "iOS"],
  [/\bAndroid\b/, "Android"],
  [/\bWindows\b/, "Windows"],
  [/\bCrOS\b/, "ChromeOS"],
  [/\b(Macintosh|Mac OS X)\b/, "macOS"],
  [/\bLinux\b/, "Linux"],
];

function match(table: [RegExp, string][], userAgent: string): string | undefined {
  return table.find(([pattern]) => pattern.test(userAgent))?.[1];
}

/** Describes a User-Agent for people: "Firefox" on "Linux", not version soup. */
export function describeUserAgent(userAgent: string): DeviceDescription {
  const os = match(systems, userAgent);
  return { browser: match(browsers, userAgent), os, mobile: os === "iOS" || os === "Android" || /\bMobile\b/.test(userAgent) };
}
