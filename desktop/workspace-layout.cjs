const { DEFAULT_EDGE_MARGIN, DEFAULT_GAP } = require("./window-layout.cjs");

function clamp(value, minimum, maximum) {
  return Math.max(minimum, Math.min(value, maximum));
}

function workspacePairBounds(workArea, calendarSizing, daySizing, {
  edgeMargin = DEFAULT_EDGE_MARGIN,
  gap = DEFAULT_GAP,
} = {}) {
  const availableWidth = Math.max(0, workArea.width - edgeMargin * 2);
  const effectiveGap = Math.min(gap, Math.max(0, availableWidth - calendarSizing.width - daySizing.width));
  const totalWidth = Math.min(availableWidth, calendarSizing.width + effectiveGap + daySizing.width);
  const groupHeight = Math.max(calendarSizing.height, daySizing.height);
  const x = workArea.x + workArea.width - edgeMargin - totalWidth;
  const y = clamp(
    workArea.y + workArea.height - edgeMargin - groupHeight,
    workArea.y + edgeMargin,
    workArea.y + workArea.height - edgeMargin - groupHeight,
  );
  const calendarWidth = Math.min(calendarSizing.width, totalWidth);
  const dayWidth = Math.min(daySizing.width, Math.max(0, totalWidth - calendarWidth - effectiveGap));

  return {
    calendar: { x, y, width: calendarWidth, height: calendarSizing.height },
    day: {
      x: x + calendarWidth + effectiveGap,
      y,
      width: dayWidth,
      height: daySizing.height,
    },
    gap: effectiveGap,
  };
}

function translatePeerBounds(sourceCurrent, sourceNext, peerCurrent) {
  return {
    ...peerCurrent,
    x: peerCurrent.x + sourceNext.x - sourceCurrent.x,
    y: peerCurrent.y + sourceNext.y - sourceCurrent.y,
  };
}

function resizePeerBounds(sourceRole, sourceNext, peerCurrent, gap = DEFAULT_GAP) {
  if (sourceRole === "calendar") {
    return {
      ...peerCurrent,
      x: sourceNext.x + sourceNext.width + gap,
      y: sourceNext.y,
    };
  }

  return {
    ...peerCurrent,
    x: sourceNext.x - gap - peerCurrent.width,
    y: sourceNext.y,
  };
}

function shiftPairIntoWorkArea(workArea, leftBounds, rightBounds, edgeMargin = DEFAULT_EDGE_MARGIN) {
  const group = {
    left: Math.min(leftBounds.x, rightBounds.x),
    top: Math.min(leftBounds.y, rightBounds.y),
    right: Math.max(leftBounds.x + leftBounds.width, rightBounds.x + rightBounds.width),
    bottom: Math.max(leftBounds.y + leftBounds.height, rightBounds.y + rightBounds.height),
  };
  const minimumX = workArea.x + edgeMargin;
  const maximumX = workArea.x + workArea.width - edgeMargin;
  const minimumY = workArea.y + edgeMargin;
  const maximumY = workArea.y + workArea.height - edgeMargin;
  let dx = 0;
  let dy = 0;

  if (group.left < minimumX) dx = minimumX - group.left;
  else if (group.right > maximumX) dx = maximumX - group.right;
  if (group.top < minimumY) dy = minimumY - group.top;
  else if (group.bottom > maximumY) dy = maximumY - group.bottom;

  return {
    left: { ...leftBounds, x: leftBounds.x + dx, y: leftBounds.y + dy },
    right: { ...rightBounds, x: rightBounds.x + dx, y: rightBounds.y + dy },
  };
}

function dayViewPairBounds(workArea, calendarBounds, daySizing, {
  edgeMargin = DEFAULT_EDGE_MARGIN,
  gap = DEFAULT_GAP,
} = {}) {
  const day = {
    x: calendarBounds.x + calendarBounds.width + gap,
    y: calendarBounds.y,
    width: daySizing.width,
    height: daySizing.height,
  };
  const shifted = shiftPairIntoWorkArea(workArea, calendarBounds, day, edgeMargin);
  return { calendar: shifted.left, day: shifted.right };
}

function calendarPairFromDayBounds(workArea, dayBounds, calendarSizing, {
  edgeMargin = DEFAULT_EDGE_MARGIN,
  gap = DEFAULT_GAP,
} = {}) {
  const calendar = {
    x: dayBounds.x - gap - calendarSizing.width,
    y: dayBounds.y,
    width: calendarSizing.width,
    height: calendarSizing.height,
  };
  const shifted = shiftPairIntoWorkArea(workArea, calendar, dayBounds, edgeMargin);
  return { calendar: shifted.left, day: shifted.right };
}

module.exports = {
  calendarPairFromDayBounds,
  dayViewPairBounds,
  resizePeerBounds,
  shiftPairIntoWorkArea,
  translatePeerBounds,
  workspacePairBounds,
};
