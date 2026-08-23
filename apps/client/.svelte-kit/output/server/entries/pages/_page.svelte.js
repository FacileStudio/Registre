import { a4 as attributes, a5 as clsx, a3 as derived, e as escape_html, a6 as head } from "../../chunks/index.js";
import { extendTailwindMerge } from "tailwind-merge";
const sizes = ["fc-xs", "fc-sm", "fc-md", "fc-lg", "fc-xl", "fc-2xl", "fc-3xl"];
const colors = [
  "fc-page",
  "fc-bg",
  "fc-surface",
  "fc-component",
  "fc-fg",
  "fc-fg-muted",
  "fc-accent",
  "fc-accent-fg",
  "fc-border",
  "fc-ring",
  "fc-scrim",
  "fc-danger",
  "fc-danger-fg",
  "fc-success",
  "fc-info",
  "fc-warning",
  "fc-owner",
  "fc-admin",
  "fc-chart-1",
  "fc-chart-2",
  "fc-chart-3",
  "fc-chart-4",
  "fc-chart-5",
  "fc-chart-6"
];
const radii = ["fc-xs", "fc-sm", "fc-md", "fc-lg", "fc-pill", "fc-full"];
const containers = ["fc-sm", "fc-md", "fc-lg", "fc-xl"];
const navWidths = ["fc-nav-collapsed", "fc-nav-expanded", "fc-nav-content", "fc-nav-item"];
const twMerge = extendTailwindMerge({
  extend: {
    classGroups: {
      "font-family": [{ font: ["fc-body", "fc-title", "fc-mono"] }],
      "font-size": [{ text: sizes }],
      "text-color": [{ text: colors }],
      "bg-color": [{ bg: colors }],
      "border-color": [{ border: colors }],
      rounded: [{ rounded: radii }],
      "max-w": [{ "max-w": containers }],
      w: [{ w: navWidths }],
      size: [{ size: navWidths }]
    }
  }
});
function Card($$renderer, $$props) {
  $$renderer.component(($$renderer2) => {
    let {
      href,
      class: className = "",
      children,
      $$slots,
      $$events,
      ...rest
    } = $$props;
    const anchorRest = derived(() => rest);
    const base = "rounded-fc-md bg-fc-component p-5";
    const classes = derived(() => twMerge(base, href && "group block transition-colors hover:bg-fc-surface focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-fc-ring", className));
    if (href) {
      $$renderer2.push("<!--[0-->");
      $$renderer2.push(`<a${attributes({ href, class: clsx(classes()), ...anchorRest() })}>`);
      children($$renderer2);
      $$renderer2.push(`<!----></a>`);
    } else {
      $$renderer2.push("<!--[-1-->");
      $$renderer2.push(`<div${attributes({ class: clsx(classes()), ...rest })}>`);
      children($$renderer2);
      $$renderer2.push(`<!----></div>`);
    }
    $$renderer2.push(`<!--]-->`);
  });
}
function Alert($$renderer, $$props) {
  $$renderer.component(($$renderer2) => {
    let {
      tone = "info",
      title,
      class: className = "",
      children,
      $$slots,
      $$events,
      ...rest
    } = $$props;
    const tones = {
      neutral: "bg-fc-surface text-fc-fg",
      info: "bg-fc-info/10 text-fc-info",
      success: "bg-fc-success/10 text-fc-success",
      warning: "bg-fc-warning/10 text-fc-warning",
      danger: "bg-fc-danger/10 text-fc-danger"
    };
    const role = derived(() => tone === "warning" || tone === "danger" ? "alert" : "status");
    const classes = derived(() => twMerge("rounded-fc-md p-3 text-fc-sm", tones[tone], className));
    $$renderer2.push(`<div${attributes({ class: clsx(classes()), role: role(), ...rest })}>`);
    if (title) {
      $$renderer2.push("<!--[0-->");
      $$renderer2.push(`<p class="font-medium mb-1">${escape_html(title)}</p>`);
    } else {
      $$renderer2.push("<!--[-1-->");
    }
    $$renderer2.push(`<!--]--> `);
    if (children) {
      $$renderer2.push("<!--[0-->");
      children($$renderer2);
      $$renderer2.push(`<!---->`);
    } else {
      $$renderer2.push("<!--[-1-->");
    }
    $$renderer2.push(`<!--]--></div>`);
  });
}
function _page($$renderer, $$props) {
  $$renderer.component(($$renderer2) => {
    let $$settled = true;
    let $$inner_renderer;
    function $$render_inner($$renderer3) {
      head("1uha8ag", $$renderer3, ($$renderer4) => {
        $$renderer4.title(($$renderer5) => {
          $$renderer5.push(`<title>Sign in — registre</title>`);
        });
      });
      $$renderer3.push(`<main class="grid min-h-dvh place-items-center bg-fc-page px-4">`);
      Card($$renderer3, {
        class: "w-full max-w-sm p-8",
        children: ($$renderer4) => {
          $$renderer4.push(`<h1 class="mb-1 text-fc-lg font-semibold text-fc-fg">Sign in to registre</h1> <p class="mb-6 text-fc-sm text-fc-fg-muted">The Facile Suite's identity provider. Sign-in starts from the app you are trying to reach.</p> `);
          {
            $$renderer4.push("<!--[0-->");
            Alert($$renderer4, {
              tone: "danger",
              title: "This sign-in link is not valid.",
              children: ($$renderer5) => {
                $$renderer5.push(`<!---->Start again from the app that sent you here.`);
              },
              $$slots: { default: true }
            });
          }
          $$renderer4.push(`<!--]-->`);
        },
        $$slots: { default: true }
      });
      $$renderer3.push(`<!----></main>`);
    }
    do {
      $$settled = true;
      $$inner_renderer = $$renderer2.copy();
      $$render_inner($$inner_renderer);
    } while (!$$settled);
    $$renderer2.subsume($$inner_renderer);
  });
}
export {
  _page as default
};
