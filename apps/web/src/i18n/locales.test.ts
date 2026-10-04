import { describe, expect, it } from "vitest";
import { intlLocale, isLocale, localeFromAcceptLanguage } from "./locales";

describe("locales", () => {
  it("detects supported locales", () => {
    expect(isLocale("es")).toBe(true);
    expect(isLocale("fr")).toBe(false);
    expect(isLocale(undefined)).toBe(false);
  });

  it.each([
    ["es-MX,es;q=0.9,en;q=0.8", "es"],
    ["fr-FR,en;q=0.5", "en"],
    ["de", "en"],
    ["", "en"],
    [null, "en"],
  ])("resolves %s to %s", (header, expected) => {
    expect(localeFromAcceptLanguage(header)).toBe(expected);
  });

  it("maps to regional formatting tags", () => {
    expect(intlLocale("es")).toBe("es-MX");
    expect(intlLocale("en")).toBe("en-US");
  });
});
