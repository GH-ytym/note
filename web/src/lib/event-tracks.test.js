import test from "node:test";
import assert from "node:assert/strict";
import { layoutEventTracks } from "./event-tracks.ts";

const segment = (id, start, end) => ({ item: { id }, start, end });
const lanes = result => result.segments.map(item => item.track);

test("first-fit reuses the leftmost free lane, including touching endpoints", () => {
  const items = [segment("a", 0, 100), segment("b", 10, 30),
    segment("c", 30, 50), segment("d", 100, 120)];
  assert.deepEqual(lanes(layoutEventTracks(items, 7)), [0, 1, 1, 0]);
});

test("overflow starts a new batch and forgets every old lane", () => {
  const items = [segment("a", 0, 1000), segment("b", 1, 1000),
    segment("c", 2, 1000), segment("d", 3, 10),
    segment("e", 4, 9), segment("f", 10, 20)];
  // d restarts lane 0; e can use lane 1 despite old b; f reuses lane 0
  // despite old a. Old events remain in the drawing list.
  const result = layoutEventTracks(items, 3);
  assert.deepEqual(lanes(result), [0, 1, 2, 0, 1, 0]);
  assert.equal(result.segments.length, items.length);
  assert.ok(items.every(item => !("track" in item)));
});

test("fully overlapping events cycle across multiple batches", () => {
  const items = Array.from({ length: 17 }, (_, i) => segment(String(i), i, 100));
  assert.deepEqual(lanes(layoutEventTracks(items, 7)),
    [0, 1, 2, 3, 4, 5, 6, 0, 1, 2, 3, 4, 5, 6, 0, 1, 2]);
});

test("input order is deterministic and empty days retain configured lanes", () => {
  const items = [segment("c", 10, 40), segment("b", 0, 20), segment("a", 0, 30)];
  assert.deepEqual(layoutEventTracks(items, 7), layoutEventTracks([...items].reverse(), 7));
  assert.deepEqual(layoutEventTracks([], 7), { segments: [], tracks: 7 });
});
