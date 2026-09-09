import test from "node:test";
import assert from "node:assert/strict";
import { appearanceTokens, contrast } from "./appearance.js";

test("text and accent ink remain readable across light, dark, and medium backgrounds", () => {
  for (const backgroundColor of ["#000000", "#FFFFFF", "#EEEADC", "#808080", "#777777", "#91A0A8", "#B065A0"]) {
    for (const themeColor of ["#000000", "#FFFFFF", "#F3B51B", "#CC99CC", "#35B7A0"]) {
      const tokens = appearanceTokens({ backgroundColor, themeColor, opacity: 95 });
      for (const surface of ["--page", "--panel", "--cell", "--cell-hover"]) {
        for (const ink of ["--text", "--muted", "--dim", "--accent-ink", "--danger"]) {
          assert.ok(contrast(tokens[ink], tokens[surface]) >= 4.5, `${backgroundColor} ${themeColor}: ${ink} on ${surface}`);
        }
      }
      assert.ok(contrast(tokens["--on-theme"], themeColor) >= 4.5);
    }
  }
});
