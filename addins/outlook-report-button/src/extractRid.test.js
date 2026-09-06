import { describe, expect, it } from "vitest";
import { extractRid } from "./extractRid.js";

describe("extractRid", () => {
  it("extracts a rid from a leading ?rid= query string", () => {
    expect(extractRid("https://example.com/?rid=AbC1234")).toBe("AbC1234");
  });

  it("extracts a rid from a &rid= joined param", () => {
    expect(
      extractRid("https://example.com/track?utm=1&rid=Zz9YxW1&other=2")
    ).toBe("Zz9YxW1");
  });

  it("returns null when no rid is present", () => {
    expect(extractRid("https://example.com/track?utm=1&other=2")).toBeNull();
  });

  it("returns null for non-string input", () => {
    expect(extractRid(undefined)).toBeNull();
    expect(extractRid(null)).toBeNull();
  });

  it("does not falsely match a rid-shaped substring lacking the query-param delimiter", () => {
    // "AbC1234" is 7 alphanumeric characters, but it isn't preceded by
    // "?rid=" or "&rid=" here, so it must not match.
    expect(extractRid("https://example.com/?other=AbC1234xyz")).toBeNull();
    // Same idea, embedded in running text with no delimiter at all.
    expect(extractRid("some text mentioning AbC1234 in passing")).toBeNull();
  });

  it("does not match when the captured group would exceed 7 characters", () => {
    // An 8-character alphanumeric run after rid= should not match via a loose
    // 7-char prefix; the \b boundary requires the alphanumeric run to end
    // exactly at 7 characters.
    expect(extractRid("https://example.com/?rid=AbC12345")).toBeNull();
  });
});
