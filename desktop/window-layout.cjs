const DEFAULT_EDGE_MARGIN = 14;
const DEFAULT_GAP = 14;

function clamp(value, minimum, maximum) {
  return Math.max(minimum, Math.min(value, maximum));
}

function fitBounds(workArea, bounds) {
  const width = Math.min(bounds.width, workArea.width);
  const height = Math.min(bounds.height, workArea.height);
  return {
    x: clamp(bounds.x, workArea.x, workArea.x + workArea.width - width),
    y: clamp(bounds.y, workArea.y, workArea.y + workArea.height - height),
    width,
    height,
  };
}

function bottomRightBounds(workArea, sizing, edgeMargin = DEFAULT_EDGE_MARGIN) {
  return fitBounds(workArea, {
    x: workArea.x + workArea.width - sizing.width - edgeMargin,
    y: workArea.y + workArea.height - sizing.height - edgeMargin,
    width: sizing.width,
    height: sizing.height,
  });
}

function aboveAnchorBounds(workArea, sizing, anchorBounds, {
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

function leftOfBounds(workArea, sizing, anchorBounds, {
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

function centeredBounds(workArea, sizing, anchorBounds = workArea) {
  return fitBounds(workArea, {
    x: anchorBounds.x + Math.round((anchorBounds.width - sizing.width) / 2),
    y: anchorBounds.y + Math.round((anchorBounds.height - sizing.height) / 2),
    width: sizing.width,
    height: sizing.height,
  });
}

module.exports = {
  DEFAULT_EDGE_MARGIN,
  DEFAULT_GAP,
  aboveAnchorBounds,
  bottomRightBounds,
  centeredBounds,
  fitBounds,
  leftOfBounds,
};
