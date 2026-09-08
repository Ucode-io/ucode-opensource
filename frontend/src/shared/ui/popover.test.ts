import { describe, expect, it } from "vitest";
import { placement } from "./popover";

const view = { width: 1000, height: 800 };

/** Кнопка 200×36 с левым верхним углом в (x, y). */
const anchor = (x: number, y: number, width = 200) => ({
  top: y,
  bottom: y + 36,
  left: x,
  right: x + width,
});

describe("placement", () => {
  it("ставит меню под кнопкой, когда снизу помещается", () => {
    const spot = placement({
      anchor: anchor(100, 100),
      width: 200,
      height: 300,
      view,
      align: "start",
    });

    expect(spot.top).toBe(140);
    expect(spot.bottom).toBeUndefined();
    expect(spot.left).toBe(100);
  });

  it("разворачивает вверх, когда снизу не помещается, а сверху места больше", () => {
    const spot = placement({
      anchor: anchor(100, 600),
      width: 200,
      height: 400,
      view,
      align: "start",
    });

    // Нижним краем к кнопке: 800 - 600 + 4.
    expect(spot.bottom).toBe(204);
    expect(spot.top).toBeUndefined();
    expect(spot.maxHeight).toBe(588);
  });

  it("оставляет меню внизу, когда сверху места ещё меньше", () => {
    const spot = placement({
      anchor: anchor(100, 300),
      width: 200,
      height: 900,
      view,
      align: "start",
    });

    expect(spot.top).toBe(340);
    expect(spot.maxHeight).toBe(452);
  });

  it("подбирает меню внутрь экрана, когда справа не помещается", () => {
    const spot = placement({
      anchor: anchor(900, 100),
      width: 300,
      height: 100,
      view,
      align: "start",
    });

    expect(spot.left).toBe(692);
  });

  it("прижимает правым краем к кнопке при align=end", () => {
    const spot = placement({
      anchor: anchor(400, 100),
      width: 300,
      height: 100,
      view,
      align: "end",
    });

    expect(spot.left).toBe(300);
  });

  it("не даёт меню уехать за левый край", () => {
    const spot = placement({
      anchor: anchor(4, 100, 40),
      width: 300,
      height: 100,
      view,
      align: "end",
    });

    expect(spot.left).toBe(8);
  });

  it("оставляет высоту даже у кнопки, прижатой к низу окна", () => {
    const spot = placement({
      anchor: anchor(100, 780),
      width: 200,
      height: 300,
      view,
      align: "start",
    });

    expect(spot.maxHeight).toBeGreaterThanOrEqual(96);
  });
});
