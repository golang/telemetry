/**
 * @license
 * Copyright 2023 The Go Authors. All rights reserved.
 * Use of this source code is governed by a BSD-style
 * license that can be found in the LICENSE file.
 */

import * as Plot from "@observablehq/plot";

import "../../../../godev/content/shared/base";

declare global {
  interface Page {
    Charts: Program[];
  }

  interface Program {
    ID: string;
    Name: string;
    Counters: Counter[];
    Active: boolean;
  }

  interface Counter {
    ID: string;
    Name: string;
    Data: Datum[];
  }

  interface Datum {
    [key: string]: any;
    Week: string;
    Program: string;
    Version: string;
    GOARCH: string;
    GOOS: string;
    GoVersion: string;
    Key: string;
    Value: number;
  }
  const Page: Page;
}

window.onload = function () {
  drawCharts();
  facetToggles();
  configSelector();
  breadcrumbController();
  sectionController();
};

// sectionController adds event listeners to the section headers
// to toggle them open and closed.
function sectionController() {
  const html = document.querySelector("html")!;
  for (const e of document.querySelectorAll("h2")) {
    e.addEventListener("click", function () {
      let closed = localStorage.getItem("closed-sections")?.split(",");
      if (closed?.includes(this.id)) {
        closed = closed.filter((v) => v !== this.id);
        const str = closed.join(",");
        localStorage.setItem("closed-sections", str);
        html.setAttribute("data-closed-sections", str);
      } else {
        closed = [this.id].concat(closed ?? []);
        const str = closed.join(",");
        localStorage.setItem("closed-sections", str);
        html.setAttribute("data-closed-sections", str);
      }
    });
  }
}

// drawCharts draws the charts using @observable/plot. It is called when
// the page is first rendered and when a facet is selected.
function drawCharts() {
  for (const program of Page.Charts ?? []) {
    for (const counter of program.Counters) {
      const xdomain = () => {
        const weeks = new Set(counter.Data.map((d) => d.Week).sort());
        const start = new Date(Array.from(weeks.values()).at(0) ?? 0);
        const end = new Date();
        const day = 1000 * 60 * 60 * 24;
        if (Math.ceil(Math.abs(end.getTime() - start.getTime()) / day) < 30) {
          start.setDate(start.getDate() - 29);
          end.setDate(end.getDate() + 1);
        }
        return [start, end];
      };

      const fy = (d: Datum) => {
        const facets: string[] = [];
        for (const i of ["Version", "GoVersion"]) {
          const params = new URLSearchParams(location.search);
          if (params.get(i) === "on") {
            facets.push(d[i] || "empty");
          }
        }
        return facets.join(" / ");
      };

      const rectYOpts: Plot.BinXInputs<Plot.RectYOptions> = {
        tip: true,
        x: (d: Datum) => new Date(d.Week),
        y: (d: Datum) => d.Value,
        interval: "week",
        fill: (d: Datum) => {
          const n = Number(d.Key);
          return isNaN(n) ? d.Key : n;
        },
        fy,
      };

      const chart = Plot.plot({
        x: {
          domain: xdomain(),
          label: "Week",
        },
        y: {
          label: "Value",
        },
        color: {
          type: "ordinal",
          legend: true,
          scheme: "Spectral",
          reverse: true,
          label: "Counter",
        },
        style: "overflow:visible;width:100%;background:transparent",
        marks: [
          Plot.rectY(counter.Data, Plot.binX({ y: "sum" }, rectYOpts)),
          Plot.ruleY([0]),
        ],
      });
      document
        .querySelector(`[data-chart-id="${counter.ID}"]`)
        ?.replaceChildren(chart);
    }
  }
}

// facetTogglers adds event listeners to the Facet component for splitting
// the charts by facet.
function facetToggles() {
  const container = document.querySelector(".js-facets");
  const els = container?.querySelectorAll<HTMLInputElement>("input") ?? [];
  for (const i of els) {
    const params = new URLSearchParams(location.search);
    i.checked = params.get(i.value) === "on";
    i.addEventListener("change", () => {
      const params = new URLSearchParams(location.search);
      params.set(i.value, i.checked ? "on" : "off");
      history.replaceState(null, "", "?" + params.toString());
      drawCharts();
    });
  }
}

// configSelector adds an event listener that reloads the page when a config
// version is selected.
function configSelector() {
  const el = document.querySelector<HTMLButtonElement>(".js-selectConfig");
  el?.addEventListener("change", () => {
    const params = new URLSearchParams(location.search);
    params.set(el.name, el.value);
    history.replaceState(null, "", "?" + params.toString());
    location.reload();
  });
}

// breadcrumbController updates the navigation header as the user scrolls
// that page displaying information about the content currently in the
// viewport.
function breadcrumbController() {
  const headings =
    document.querySelectorAll<HTMLHeadingElement>("h1, h2, h3, h4");
  const callback = debounce(() => {
    let above: HTMLHeadingElement[] = [];
    for (const h of headings) {
      const rect = h.getBoundingClientRect();
      if (rect.height && rect.top < 80) {
        above.unshift(h);
      }
    }
    if (above.length < 2) {
      above = [];
    }
    let threshold = Infinity;
    const els: HTMLHeadingElement[] = [];
    for (const h of above) {
      const level = Number(h.tagName[1]);
      if (level < threshold) {
        threshold = level;
        els.unshift(h);
      }
    }
    const breadcrumb = document.querySelector(".js-breadcrumb ol");
    const items = [];
    for (const h of els) {
      breadcrumb?.replaceChildren;
      const li = document.createElement("li");
      const a = document.createElement("a");
      a.href = `#${h.id}`;
      a.innerText = h.getAttribute('data-label') ?? h.innerText;
      li.appendChild(a);
      items.push(li);
    }
    breadcrumb?.replaceChildren(...items);
  }, 100);

  const observer = new IntersectionObserver(callback);
  for (const h of headings) {
    observer.observe(h);
  }
}

function debounce<T extends (...args: unknown[]) => unknown>(
  callback: T,
  wait: number
) {
  let timeout: number;
  return (...args: unknown[]) => {
    clearTimeout(timeout);
    timeout = setTimeout(() => callback(...args), wait);
  };
}

export {};
