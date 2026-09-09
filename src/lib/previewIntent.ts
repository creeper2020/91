export type PreviewIntentInput = {
  previewEnabled: boolean;
  pointerType?: string;
  canHover: boolean;
  previewActive: boolean;
};

export const TOUCH_PREVIEW_DELAY_MS = 200;

export function shouldInterceptPreviewTap(input: PreviewIntentInput): boolean {
  return (
    input.previewEnabled &&
    isTouchLike(input.pointerType, input.canHover) &&
    !input.previewActive
  );
}

export function shouldStartInstantPreview(input: {
  pointerType?: string;
}): boolean {
  return input.pointerType === "touch";
}

function isTouchLike(pointerType: string | undefined, canHover: boolean): boolean {
  if (pointerType === "mouse") return false;
  if (pointerType === "touch") return true;
  return !canHover;
}
