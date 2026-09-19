import type { Rectangle } from "electron";
type Size = Pick<Rectangle, "width" | "height">;
const DEFAULT_EDGE_MARGIN = 14;
const DEFAULT_GAP = 14;

function clamp(value: number, minimum: number, maximum: number) {
  return Math.max(minimum, Math.min(value, maximum));
}

function fitBounds(workArea: Rectangle, bounds: Rectangle) {
  const width = Math.min(bounds.width, workArea.width);
  const height = Math.min(bounds.height, workArea.height);
  return {
    x: clamp(bounds.x, workArea.x, workArea.x + workArea.width - width),
    y: clamp(bounds.y, workArea.y, workArea.y + workArea.height - height),
    width,
    height,
  };
}

function bottomRightBounds(workArea: Rectangle, sizing: Size, edgeMargin = DEFAULT_EDGE_MARGIN) {
  return fitBounds(workArea, {
    x: workArea.x + workArea.width - sizing.width - edgeMargin,
    y: workArea.y + workArea.height - sizing.height - edgeMargin,
    width: sizing.width,
    height: sizing.height,
  });
}

function aboveAnchorBounds(workArea: Rectangle, sizing: Size, anchorBounds: Rectangle, {
  edgeMargin = DEFAULT_EDGE_MARGIN,
  gap = DEFAULT_GAP,
  offset = 0,
} = {}) {
  return fitBounds(workArea, {
    x: workArea.x + workArea.width - sizing.width - edgeMargin - offset,
    y: anchorBounds.y - sizing.height - gap - offset,
    width: sizing.width,
    height: sizing.height,
  });
}

function leftOfBounds(workArea: Rectangle, sizing: Size, anchorBounds: Rectangle, {
  gap = DEFAULT_GAP,
  verticalOffset = 0,
} = {}) {
  return fitBounds(workArea, {
    x: anchorBounds.x - sizing.width - gap,
    y: anchorBounds.y + Math.round((anchorBounds.height - sizing.height) / 2) + verticalOffset,
    width: sizing.width,
    height: sizing.height,
  });
}

function centeredBounds(workArea: Rectangle, sizing: Size, anchorBounds: Rectangle = workArea) {
  return fitBounds(workArea, {
    x: anchorBounds.x + Math.round((anchorBounds.width - sizing.width) / 2),
    y: anchorBounds.y + Math.round((anchorBounds.height - sizing.height) / 2),
    width: sizing.width,
    height: sizing.height,
  });
}

export {
  DEFAULT_EDGE_MARGIN,
  DEFAULT_GAP,
  aboveAnchorBounds,
  bottomRightBounds,
  centeredBounds,
  fitBounds,
  leftOfBounds,
};
