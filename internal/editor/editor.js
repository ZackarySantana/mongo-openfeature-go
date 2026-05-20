/* ============================================================
   Flag editor client script
   - Targeted DOM updates (no full re-render on edits)
   - Grouped "Add rule" popover
   - Composite nesting accents + collapse
   - JSON inline errors + Format helper
   - Debounced sync to hidden <textarea>
   - Two-click confirm-delete pattern (with htmx)
   - Toast bridge (HX-Trigger -> #toast-region)
   - Unsaved-changes guard on the edit form
   - Live filter for the flag list
   ============================================================ */

(function () {
    "use strict";

    const RULE_TYPES = [
        {
            label: "Identity & matching",
            options: [
                { type: "ExactMatchRule", desc: "Key equals a specific value" },
                { type: "RegexRule", desc: "Key matches a regular expression" },
                { type: "ExistsRule", desc: "Key is present in the context" },
                { type: "PrefixRule", desc: "Key starts with a prefix" },
                { type: "SuffixRule", desc: "Key ends with a suffix" },
                { type: "ContainsRule", desc: "Key contains a substring" },
                { type: "InListRule", desc: "Key is in a list of values" },
            ],
        },
        {
            label: "Numeric",
            options: [
                { type: "RangeRule", desc: "Numeric key within a range" },
                { type: "FractionalRule", desc: "Random percentage rollout" },
            ],
        },
        {
            label: "Network & geo",
            options: [
                { type: "IPRangeRule", desc: "Key matches a CIDR range" },
                { type: "GeoFenceRule", desc: "Lat/lng within a radius" },
            ],
        },
        {
            label: "Time",
            options: [
                { type: "DateTimeRule", desc: "Within an RFC3339 window" },
                { type: "SemVerRule", desc: "Semantic version constraint" },
                { type: "CronRule", desc: "Active during a cron window" },
            ],
        },
        {
            label: "Composite",
            options: [
                { type: "AndRule", desc: "All child rules must match" },
                { type: "OrRule", desc: "Any child rule must match" },
                { type: "NotRule", desc: "Invert a child rule" },
            ],
        },
        {
            label: "Override",
            options: [
                {
                    type: "OverrideRule",
                    desc: "Force a variant regardless of rules",
                },
            ],
        },
    ];

    const COMPOSITE_TYPES = new Set(["andRule", "orRule", "notRule"]);
    const DOC_BASE =
        "https://github.com/ZackarySantana/mongo-openfeature-go?tab=readme-ov-file";

    // Convert "ExactMatchRule" -> "exactMatchRule" (lower-camel for JSON key).
    function toJsonKey(typeName) {
        if (typeName === "IPRangeRule") return "ipRangeRule";
        if (typeName === "SemVerRule") return "semVerRule";
        return typeName.charAt(0).toLowerCase() + typeName.slice(1);
    }

    function debounce(fn, ms) {
        let t;
        return function () {
            clearTimeout(t);
            const args = arguments;
            t = setTimeout(() => fn.apply(null, args), ms);
        };
    }

    /* ============================================================
       Toast bridge
       ============================================================ */

    function showToast(opts) {
        opts = opts || {};
        const region = document.getElementById("toast-region");
        if (!region) return;
        const el = document.createElement("div");
        el.className =
            "toast toast--" + (opts.kind === "error" ? "error" : "success");
        const body = document.createElement("div");
        if (opts.title) {
            const t = document.createElement("div");
            t.className = "toast__title";
            t.textContent = opts.title;
            body.appendChild(t);
        }
        if (opts.body) {
            const b = document.createElement("div");
            b.className = "toast__body";
            b.textContent = opts.body;
            body.appendChild(b);
        }
        el.appendChild(body);
        region.appendChild(el);
        setTimeout(() => {
            el.classList.add("is-leaving");
            setTimeout(() => el.remove(), 220);
        }, 3200);
    }

    // htmx fires CustomEvent for each name in HX-Trigger.
    document.body &&
        document.body.addEventListener("showToast", function (evt) {
            showToast(evt.detail || {});
        });
    document.addEventListener("DOMContentLoaded", function () {
        document.body.addEventListener("showToast", function (evt) {
            showToast(evt.detail || {});
        });
    });

    /* ============================================================
       Confirm-delete button (works standalone or with htmx)
       ============================================================ */

    function wireConfirmButton(btn) {
        if (btn.__confirmWired) return;
        btn.__confirmWired = true;
        const originalText = btn.textContent.trim();
        let armTimer = null;

        function disarm() {
            btn.dataset.confirm = "idle";
            btn.textContent = originalText;
            if (armTimer) {
                clearTimeout(armTimer);
                armTimer = null;
            }
        }

        btn.addEventListener(
            "click",
            function (evt) {
                if (btn.dataset.confirm !== "armed") {
                    evt.preventDefault();
                    evt.stopImmediatePropagation();
                    btn.dataset.confirm = "armed";
                    btn.textContent = "Click again to delete";
                    armTimer = setTimeout(disarm, 3000);
                    return;
                }
                disarm();
                btn.dispatchEvent(
                    new CustomEvent("confirmed", { bubbles: true }),
                );
                // Visual fade for list rows after htmx removes them.
                const row = btn.closest(".flag-row");
                if (row) row.classList.add("is-removing");
            },
            true,
        );

        btn.addEventListener("blur", function () {
            if (btn.dataset.confirm === "armed") disarm();
        });
    }

    function wireAllConfirmButtons(root) {
        (root || document)
            .querySelectorAll(".confirm-btn")
            .forEach(wireConfirmButton);
    }

    /* ============================================================
       List page: click whole row to open flag
       ============================================================ */

    function wireFlagRow(row) {
        if (row.__flagRowWired) return;
        row.__flagRowWired = true;
        const href = row.dataset.href;
        if (!href) return;

        function navigate() {
            window.location.href = href;
        }

        row.addEventListener("click", function (evt) {
            if (
                evt.target.closest("button, a, input, textarea, select, label")
            ) {
                return;
            }
            navigate();
        });
        row.addEventListener("keydown", function (evt) {
            if (evt.target !== row) return;
            if (evt.key === "Enter" || evt.key === " ") {
                evt.preventDefault();
                navigate();
            }
        });
    }

    function wireAllFlagRows(root) {
        (root || document)
            .querySelectorAll(".flag-row[data-href]")
            .forEach(wireFlagRow);
    }

    /* ============================================================
       List page: live search filter
       ============================================================ */

    function setupListSearch() {
        const box = document.getElementById("search-box");
        const list = document.getElementById("flag-list");
        if (!box || !list) return;
        const rows = Array.from(list.querySelectorAll(".flag-row"));
        box.addEventListener("input", function () {
            const q = box.value.trim().toLowerCase();
            rows.forEach((row) => {
                const name = (row.dataset.name || "").toLowerCase();
                row.style.display = !q || name.includes(q) ? "" : "none";
            });
        });
    }

    /* ============================================================
       Edit page: default-value JSON validator + Format button
       ============================================================ */

    function setupJsonFieldBlocks() {
        document.querySelectorAll("[data-json-field]").forEach((field) => {
            const ta = field.querySelector("textarea, input");
            const formatBtn = field.querySelector("[data-json-format]");
            if (!ta) return;

            function validate() {
                if (ta.value.trim() === "") {
                    field.classList.remove("is-invalid");
                    return true;
                }
                try {
                    JSON.parse(ta.value);
                    field.classList.remove("is-invalid");
                    return true;
                } catch (e) {
                    field.classList.add("is-invalid");
                    return false;
                }
            }
            ta.addEventListener("input", validate);
            ta.addEventListener("blur", validate);
            validate();

            if (formatBtn) {
                formatBtn.addEventListener("click", function () {
                    try {
                        const parsed = JSON.parse(ta.value);
                        ta.value = JSON.stringify(parsed, null, 2);
                        field.classList.remove("is-invalid");
                        ta.dispatchEvent(new Event("input", { bubbles: true }));
                    } catch (e) {
                        field.classList.add("is-invalid");
                    }
                });
            }
        });
    }

    /* ============================================================
       Edit page: form dirty-tracker + beforeunload guard
       ============================================================ */

    function setupFormDirtyGuard() {
        const form = document.querySelector("form[data-flag-form]");
        if (!form) return;
        let dirty = false;

        function markDirty() {
            dirty = true;
        }
        form.addEventListener("input", markDirty);
        form.addEventListener("change", markDirty);

        window.addEventListener("beforeunload", function (e) {
            if (!dirty) return;
            e.preventDefault();
            e.returnValue = "";
            return "";
        });

        // Reset on successful htmx save.
        document.body.addEventListener("htmx:afterRequest", function (evt) {
            const xhr = evt.detail && evt.detail.xhr;
            const elt = evt.detail && evt.detail.elt;
            if (!xhr || !elt) return;
            if (elt !== form && !form.contains(elt)) return;
            if (xhr.status >= 200 && xhr.status < 400) {
                dirty = false;
            }
        });

        // Internal hook: rule-builder can flag dirty too.
        form.__markDirty = markDirty;
    }

    /* ============================================================
       Rule builder
       ============================================================ */

    function setupRuleBuilder() {
        const textarea = document.getElementById("rules");
        const container = document.getElementById("rule-builder");
        if (!textarea || !container) return;

        textarea.style.display = "none";

        let state = [];
        try {
            const parsed = JSON.parse(textarea.value);
            if (Array.isArray(parsed)) state = parsed;
        } catch (e) {
            console.error("Invalid initial rules JSON:", e);
        }

        const form = textarea.closest("form");
        const markDirty =
            form && form.__markDirty ? form.__markDirty : function () {};

        const sync = debounce(function () {
            textarea.value = JSON.stringify(state, null, 2);
        }, 120);

        function syncNow() {
            textarea.value = JSON.stringify(state, null, 2);
        }

        // Touch immediately so the hidden textarea always has clean JSON.
        syncNow();

        let nextRuleId = 1;

        function ensureRuleId(ruleData) {
            if (!ruleData.__tocId) {
                ruleData.__tocId = "rule-" + nextRuleId++;
            }
            return ruleData.__tocId;
        }

        function updateRuleTocMeta(el, ruleData) {
            const ruleTypeKey = Object.keys(ruleData)[0];
            const rule = ruleData[ruleTypeKey];
            let meta = "";
            if (COMPOSITE_TYPES.has(ruleTypeKey)) {
                meta = computeVariant(ruleData);
            } else if (rule.VariantID) {
                meta = String(rule.VariantID);
            }
            el.dataset.tocType = ruleTypeKey;
            el.dataset.tocMeta = meta;
        }

        function getRuleDepth(ruleEl) {
            let depth = 0;
            let el = ruleEl.parentElement;
            while (el && el.id !== "rule-builder") {
                if (el.classList.contains("rule-list--nested")) depth++;
                el = el.parentElement;
            }
            return depth;
        }

        function refreshRuleTOC() {
            const tocNav = document.getElementById("rule-toc");
            const builder = document.getElementById("rule-builder");
            if (!tocNav || !builder) return;

            const rules = builder.querySelectorAll(".rule");
            tocNav.innerHTML = "";

            if (!rules.length) {
                const p = document.createElement("p");
                p.className = "rule-toc__empty";
                p.id = "rule-toc-empty";
                p.textContent = "No rules yet";
                tocNav.appendChild(p);
                return;
            }

            const list = document.createElement("ul");
            list.className = "rule-toc__list";

            let index = 0;
            rules.forEach(function (el) {
                if (el.__ruleData) updateRuleTocMeta(el, el.__ruleData);
                index++;

                const depth = getRuleDepth(el);
                const type = el.dataset.tocType || el.dataset.type || "rule";
                const meta = el.dataset.tocMeta || "";

                const li = document.createElement("li");
                li.className = "rule-toc__item";
                li.style.setProperty("--toc-depth", String(depth));

                const link = document.createElement("a");
                link.href = "#" + el.id;
                link.className = "rule-toc__link";
                link.setAttribute("data-depth", String(depth));

                const idxSpan = document.createElement("span");
                idxSpan.className = "rule-toc__index";
                idxSpan.textContent = String(index);

                const typeSpan = document.createElement("span");
                typeSpan.className = "rule-toc__type";
                typeSpan.textContent = type;

                link.appendChild(idxSpan);
                link.appendChild(typeSpan);
                if (meta) {
                    const metaSpan = document.createElement("span");
                    metaSpan.className = "rule-toc__meta";
                    metaSpan.textContent = meta;
                    link.appendChild(metaSpan);
                }

                link.addEventListener("click", function (evt) {
                    evt.preventDefault();
                    el.scrollIntoView({ behavior: "smooth", block: "start" });
                    el.classList.remove("rule--toc-flash");
                    void el.offsetWidth;
                    el.classList.add("rule--toc-flash");
                    window.setTimeout(function () {
                        el.classList.remove("rule--toc-flash");
                    }, 1200);
                });

                li.appendChild(link);
                list.appendChild(li);
            });

            tocNav.appendChild(list);
            applyRuleSearch();
        }

        function ruleSearchText(el) {
            const type = (
                el.dataset.tocType ||
                el.dataset.type ||
                ""
            ).toLowerCase();
            const meta = (el.dataset.tocMeta || "").toLowerCase();
            const body = el.querySelector(":scope > .rule__body");
            if (!body) return (type + " " + meta).trim();

            // Only this rule's fields — exclude nested rules and the "Add rule"
            // picker menu (which lists every type name, e.g. "RegexRule").
            const clone = body.cloneNode(true);
            clone.querySelectorAll(".rule, .add-rule").forEach(function (n) {
                n.remove();
            });
            return (type + " " + meta + " " + clone.textContent).toLowerCase();
        }

        function applyRuleSearch() {
            const builder = document.getElementById("rule-builder");
            const box = document.getElementById("rules-search-box");
            const emptyMsg = document.getElementById("rules-search-empty");
            if (!builder) return;

            const q = box ? box.value.trim().toLowerCase() : "";
            const rules = Array.from(builder.querySelectorAll(".rule"));
            const matched = new Set();

            if (!q) {
                rules.forEach(function (el) {
                    el.classList.remove("rule--search-hidden");
                });
                document
                    .querySelectorAll(".rule-toc__item")
                    .forEach(function (item) {
                        item.classList.remove("rule-toc__item--search-hidden");
                    });
                if (emptyMsg) emptyMsg.hidden = true;
                return;
            }

            rules.forEach(function (el) {
                if (ruleSearchText(el).includes(q)) matched.add(el);
            });

            rules.forEach(function (el) {
                if (!matched.has(el)) return;
                let parent = el.parentElement;
                while (parent && parent !== builder) {
                    if (parent.classList.contains("rule")) {
                        matched.add(parent);
                    }
                    parent = parent.parentElement;
                }
            });

            let visibleCount = 0;
            rules.forEach(function (el) {
                const show = matched.has(el);
                el.classList.toggle("rule--search-hidden", !show);
                if (show) visibleCount++;
            });

            document
                .querySelectorAll(".rule-toc__item")
                .forEach(function (item) {
                    const link = item.querySelector(".rule-toc__link");
                    const href = link && link.getAttribute("href");
                    const id = href ? href.slice(1) : "";
                    const ruleEl = id ? document.getElementById(id) : null;
                    const show = ruleEl && matched.has(ruleEl);
                    item.classList.toggle(
                        "rule-toc__item--search-hidden",
                        !show,
                    );
                });

            if (emptyMsg) {
                emptyMsg.hidden = visibleCount > 0;
            }
        }

        const scheduleTocRefresh = debounce(refreshRuleTOC, 80);

        function notifyChange() {
            sync();
            markDirty();
            scheduleTocRefresh();
        }

        // ----- Field factories -----

        function field(label, child, opts) {
            opts = opts || {};
            const wrap = document.createElement("div");
            wrap.className = "field";
            if (label) {
                const l = document.createElement("label");
                l.className = "field__label";
                l.textContent = label;
                wrap.appendChild(l);
            }
            wrap.appendChild(child);
            if (opts.error) {
                const e = document.createElement("span");
                e.className = "field__error";
                e.textContent = opts.error;
                wrap.appendChild(e);
            }
            return wrap;
        }

        function textField(label, obj, key, opts) {
            opts = opts || {};
            const input = document.createElement("input");
            input.className = "input";
            input.type = "text";
            input.value = obj[key] != null ? obj[key] : "";
            input.addEventListener("input", function () {
                obj[key] = input.value;
                notifyChange();
                if (key === "VariantID" && opts.onVariantChange)
                    opts.onVariantChange();
            });
            return field(label, input);
        }

        function numberField(label, obj, key) {
            const input = document.createElement("input");
            input.className = "input";
            input.type = "number";
            input.step = "any";
            input.value = obj[key] != null ? obj[key] : 0;
            input.addEventListener("input", function () {
                const v = parseFloat(input.value);
                obj[key] = isNaN(v) ? 0 : v;
                notifyChange();
            });
            return field(label, input);
        }

        function checkboxField(label, obj, key) {
            const wrap = document.createElement("div");
            wrap.className = "field";
            const lbl = document.createElement("label");
            lbl.className = "checkbox";
            const input = document.createElement("input");
            input.type = "checkbox";
            input.checked = !!obj[key];
            input.addEventListener("change", function () {
                obj[key] = input.checked;
                notifyChange();
            });
            const span = document.createElement("span");
            span.textContent = label;
            lbl.appendChild(input);
            lbl.appendChild(span);
            wrap.appendChild(lbl);
            return wrap;
        }

        function dateTimeField(label, obj, key) {
            const input = document.createElement("input");
            input.className = "input input--mono";
            input.type = "text";
            input.placeholder = "YYYY-MM-DDTHH:MM:SSZ";
            input.value = obj[key] != null ? obj[key] : "";
            input.addEventListener("input", function () {
                obj[key] = input.value;
                notifyChange();
            });
            return field(label, input);
        }

        function jsonField(label, obj, key) {
            const wrap = document.createElement("div");
            wrap.className = "field";

            const labelRow = document.createElement("div");
            labelRow.className = "field__label-row";
            const lbl = document.createElement("label");
            lbl.className = "field__label";
            lbl.textContent = label;
            labelRow.appendChild(lbl);

            const formatBtn = document.createElement("button");
            formatBtn.type = "button";
            formatBtn.className = "btn btn--ghost btn--sm";
            formatBtn.textContent = "Format";
            labelRow.appendChild(formatBtn);

            wrap.appendChild(labelRow);

            const ta = document.createElement("textarea");
            ta.className = "textarea textarea--mono";
            ta.spellcheck = false;
            ta.rows = 3;
            let initial = "";
            try {
                initial =
                    obj[key] === undefined
                        ? ""
                        : JSON.stringify(obj[key], null, 2);
            } catch (e) {
                initial = "";
            }
            ta.value = initial;

            const err = document.createElement("span");
            err.className = "field__error";
            err.textContent = "Invalid JSON.";

            function validate() {
                if (ta.value.trim() === "") {
                    delete obj[key];
                    wrap.classList.remove("is-invalid");
                    notifyChange();
                    return;
                }
                try {
                    obj[key] = JSON.parse(ta.value);
                    wrap.classList.remove("is-invalid");
                    notifyChange();
                } catch (e) {
                    wrap.classList.add("is-invalid");
                }
            }

            ta.addEventListener("input", validate);
            formatBtn.addEventListener("click", function () {
                try {
                    const parsed = JSON.parse(ta.value);
                    ta.value = JSON.stringify(parsed, null, 2);
                    wrap.classList.remove("is-invalid");
                } catch (e) {
                    wrap.classList.add("is-invalid");
                }
            });

            wrap.appendChild(ta);
            wrap.appendChild(err);
            return wrap;
        }

        function computedPill(value) {
            const wrap = document.createElement("div");
            wrap.className = "field";
            const lbl = document.createElement("label");
            lbl.className = "field__label";
            lbl.textContent = "Computed VariantID";
            const pill = document.createElement("div");
            pill.className = "computed-pill";
            pill.textContent = value || "—";
            wrap.appendChild(lbl);
            wrap.appendChild(pill);
            return { wrap: wrap, pill: pill };
        }

        // ----- Computed variant helpers -----

        function computeVariant(ruleData) {
            const k = Object.keys(ruleData)[0];
            const r = ruleData[k];
            switch (k) {
                case "andRule":
                    if (!r.Rules || !r.Rules.length) return "&()";
                    return "&(" + r.Rules.map(computeVariant).join("+") + ")";
                case "orRule":
                    if (!r.Rules || !r.Rules.length) return "|()";
                    return "|(" + r.Rules.map(computeVariant).join("+") + ")";
                case "notRule":
                    if (!r.Rule) return "!()";
                    return "!(" + computeVariant(r.Rule) + ")";
                default:
                    return r.VariantID || "";
            }
        }

        // ----- Rule list / element rendering -----

        function renderList(rulesArray, options) {
            options = options || {};
            const listEl = document.createElement("div");
            listEl.className =
                "rule-list" + (options.nested ? " rule-list--nested" : "");

            // Track the "Add rule" button so we can insert new rule elements
            // right before it without rebuilding the whole list (which would
            // wipe collapsed state, focus, and JSON-field validity).
            let addBtn = null;

            function makeRuleEl(ruleData) {
                return renderRule(ruleData, {
                    hideValueData: options.hideValueData,
                    hidePriority: options.hidePriority,
                    onVariantChange: options.onVariantChange,
                    onRemove: function () {
                        const i = rulesArray.indexOf(ruleData);
                        if (i >= 0) rulesArray.splice(i, 1);
                        if (options.onVariantChange) options.onVariantChange();
                        notifyChange();
                    },
                });
            }

            if (rulesArray && rulesArray.length) {
                rulesArray.forEach((ruleData) => {
                    listEl.appendChild(makeRuleEl(ruleData));
                });
            }

            if (!options.disableAdd) {
                addBtn = buildAddRule(function (newRule) {
                    rulesArray.push(newRule);
                    const el = makeRuleEl(newRule);
                    listEl.insertBefore(el, addBtn);
                    if (options.onVariantChange) options.onVariantChange();
                    notifyChange();
                });
                listEl.appendChild(addBtn);
            }

            return listEl;
        }

        function renderRule(ruleData, opts) {
            opts = opts || {};
            const ruleTypeKey = Object.keys(ruleData)[0];
            const rule = ruleData[ruleTypeKey];

            const wrapper = document.createElement("div");
            wrapper.className = "rule";
            wrapper.dataset.type = ruleTypeKey;
            wrapper.__ruleData = ruleData;
            wrapper.id = ensureRuleId(ruleData);

            // ---- Header ----
            const header = document.createElement("div");
            header.className = "rule__header";
            header.setAttribute("role", "button");
            header.setAttribute("tabindex", "0");
            header.setAttribute("aria-label", "Toggle rule details");

            function toggleCollapsed() {
                wrapper.classList.toggle("is-collapsed");
            }
            header.addEventListener("click", function (evt) {
                // Don't toggle when the click started on an interactive child
                // (link, button, input, etc.) — let those handle themselves.
                if (
                    evt.target.closest(
                        "a, button, input, textarea, select, label",
                    )
                ) {
                    return;
                }
                toggleCollapsed();
            });
            header.addEventListener("keydown", function (evt) {
                if (evt.target !== header) return;
                if (evt.key === "Enter" || evt.key === " ") {
                    evt.preventDefault();
                    toggleCollapsed();
                }
            });

            const title = document.createElement("div");
            title.className = "rule__title";

            const collapseBtn = document.createElement("button");
            collapseBtn.type = "button";
            collapseBtn.className = "rule__collapse";
            collapseBtn.setAttribute("aria-label", "Toggle rule details");
            collapseBtn.innerHTML =
                '<svg xmlns="http://www.w3.org/2000/svg" width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round"><polyline points="6 9 12 15 18 9"/></svg>';
            collapseBtn.addEventListener("click", function (evt) {
                evt.stopPropagation();
                toggleCollapsed();
            });

            const typeLink = document.createElement("a");
            typeLink.className = "rule__type-link";
            typeLink.href = DOC_BASE + "#" + ruleTypeKey.toLowerCase();
            typeLink.target = "_blank";
            typeLink.rel = "noopener";
            typeLink.title = "View documentation for " + ruleTypeKey;
            typeLink.textContent = ruleTypeKey;

            title.appendChild(collapseBtn);
            title.appendChild(typeLink);
            header.appendChild(title);

            const actions = document.createElement("div");
            actions.className = "rule__actions";
            const deleteBtn = document.createElement("button");
            deleteBtn.type = "button";
            deleteBtn.className = "btn btn--ghost btn--sm";
            deleteBtn.setAttribute("aria-label", "Delete rule");
            deleteBtn.innerHTML =
                '<svg xmlns="http://www.w3.org/2000/svg" width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><polyline points="3 6 5 6 21 6"/><path d="M19 6l-1 14a2 2 0 0 1-2 2H8a2 2 0 0 1-2-2L5 6"/><path d="M10 11v6"/><path d="M14 11v6"/><path d="M9 6V4a2 2 0 0 1 2-2h2a2 2 0 0 1 2 2v2"/></svg>';
            deleteBtn.addEventListener("click", function () {
                wrapper.classList.add("is-leaving");
                setTimeout(function () {
                    wrapper.remove();
                    if (opts.onRemove) opts.onRemove();
                }, 160);
            });
            actions.appendChild(deleteBtn);
            header.appendChild(actions);
            wrapper.appendChild(header);

            // ---- Body ----
            const body = document.createElement("div");
            body.className = "rule__body";
            wrapper.appendChild(body);

            // Computed variant pill ref for composites
            let pillRef = null;
            function refreshPill() {
                if (pillRef)
                    pillRef.pill.textContent = computeVariant(ruleData) || "—";
                scheduleTocRefresh();
                if (opts.onVariantChange) opts.onVariantChange();
            }

            // Per-type body fields
            switch (ruleTypeKey) {
                case "exactMatchRule":
                    body.appendChild(textField("Key", rule, "Key"));
                    body.appendChild(textField("KeyValue", rule, "KeyValue"));
                    break;
                case "regexRule":
                    body.appendChild(textField("Key", rule, "Key"));
                    body.appendChild(
                        textField("RegexpPattern", rule, "RegexpPattern"),
                    );
                    break;
                case "existsRule":
                    body.appendChild(textField("Key", rule, "Key"));
                    break;
                case "fractionalRule":
                    body.appendChild(textField("Key", rule, "Key"));
                    body.appendChild(
                        numberField("Percentage", rule, "Percentage"),
                    );
                    break;
                case "rangeRule":
                    body.appendChild(textField("Key", rule, "Key"));
                    body.appendChild(numberField("Min", rule, "Min"));
                    body.appendChild(numberField("Max", rule, "Max"));
                    body.appendChild(
                        checkboxField("ExclusiveMin", rule, "ExclusiveMin"),
                    );
                    body.appendChild(
                        checkboxField("ExclusiveMax", rule, "ExclusiveMax"),
                    );
                    break;
                case "inListRule":
                    body.appendChild(textField("Key", rule, "Key"));
                    body.appendChild(
                        jsonField("Items (JSON array)", rule, "Items"),
                    );
                    break;
                case "prefixRule":
                    body.appendChild(textField("Key", rule, "Key"));
                    body.appendChild(textField("Prefix", rule, "Prefix"));
                    break;
                case "suffixRule":
                    body.appendChild(textField("Key", rule, "Key"));
                    body.appendChild(textField("Suffix", rule, "Suffix"));
                    break;
                case "containsRule":
                    body.appendChild(textField("Key", rule, "Key"));
                    body.appendChild(textField("Substring", rule, "Substring"));
                    break;
                case "ipRangeRule":
                    body.appendChild(textField("Key", rule, "Key"));
                    body.appendChild(
                        jsonField("CIDRs (JSON array)", rule, "CIDRs"),
                    );
                    break;
                case "geoFenceRule":
                    body.appendChild(textField("LatKey", rule, "LatKey"));
                    body.appendChild(textField("LngKey", rule, "LngKey"));
                    body.appendChild(
                        numberField("LatCenter", rule, "LatCenter"),
                    );
                    body.appendChild(
                        numberField("LngCenter", rule, "LngCenter"),
                    );
                    body.appendChild(
                        numberField("RadiusMeters", rule, "RadiusMeters"),
                    );
                    break;
                case "dateTimeRule":
                    body.appendChild(textField("Key", rule, "Key"));
                    body.appendChild(
                        dateTimeField("After (RFC3339)", rule, "After"),
                    );
                    body.appendChild(
                        dateTimeField("Before (RFC3339)", rule, "Before"),
                    );
                    break;
                case "semVerRule":
                    body.appendChild(textField("Key", rule, "Key"));
                    body.appendChild(
                        textField("Constraint", rule, "Constraint"),
                    );
                    break;
                case "cronRule":
                    body.appendChild(textField("Key", rule, "Key"));
                    body.appendChild(textField("CronSpec", rule, "CronSpec"));
                    body.appendChild(
                        numberField("Duration (nanoseconds)", rule, "Duration"),
                    );
                    break;
                case "andRule":
                case "orRule": {
                    if (!Array.isArray(rule.Rules)) rule.Rules = [];
                    body.appendChild(
                        renderList(rule.Rules, {
                            nested: true,
                            hideValueData: true,
                            hidePriority: true,
                            onVariantChange: refreshPill,
                        }),
                    );
                    break;
                }
                case "notRule": {
                    const nestedWrap = document.createElement("div");
                    nestedWrap.className = "rule-list rule-list--nested";

                    function renderNot() {
                        nestedWrap.innerHTML = "";
                        if (rule.Rule) {
                            nestedWrap.appendChild(
                                renderRule(rule.Rule, {
                                    hideValueData: true,
                                    hidePriority: true,
                                    onVariantChange: refreshPill,
                                    onRemove: function () {
                                        rule.Rule = null;
                                        renderNot();
                                        refreshPill();
                                        notifyChange();
                                    },
                                }),
                            );
                        } else {
                            nestedWrap.appendChild(
                                buildAddRule(function (newRule) {
                                    rule.Rule = newRule;
                                    renderNot();
                                    refreshPill();
                                }),
                            );
                        }
                    }
                    renderNot();
                    body.appendChild(nestedWrap);
                    break;
                }
                case "overrideRule":
                    // No type-specific fields; just VariantID + Priority + ValueData below.
                    break;
            }

            // Variant / computed pill
            if (COMPOSITE_TYPES.has(ruleTypeKey)) {
                pillRef = computedPill(computeVariant(ruleData));
                body.appendChild(pillRef.wrap);
            } else {
                body.appendChild(
                    textField("VariantID", rule, "VariantID", {
                        onVariantChange: function () {
                            if (opts.onVariantChange) opts.onVariantChange();
                        },
                    }),
                );
            }

            // Priority
            if (!opts.hidePriority) {
                body.appendChild(numberField("Priority", rule, "Priority"));
            }

            // ValueData (always available on composites; otherwise hidden on nested children)
            if (!opts.hideValueData || COMPOSITE_TYPES.has(ruleTypeKey)) {
                body.appendChild(
                    jsonField("ValueData (JSON)", rule, "ValueData"),
                );
            }

            return wrapper;
        }

        // ----- Add-rule popover -----

        function buildAddRule(onPick) {
            const wrap = document.createElement("div");
            wrap.className = "add-rule";

            const btn = document.createElement("button");
            btn.type = "button";
            btn.className = "btn";
            btn.innerHTML =
                '<svg xmlns="http://www.w3.org/2000/svg" width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round"><path d="M12 5v14"/><path d="M5 12h14"/></svg> Add rule';

            const menu = document.createElement("div");
            menu.className = "add-rule__menu";
            menu.setAttribute("role", "menu");

            RULE_TYPES.forEach((group) => {
                const groupEl = document.createElement("div");
                groupEl.className = "add-rule__group";
                const label = document.createElement("div");
                label.className = "add-rule__group-label";
                label.textContent = group.label;
                groupEl.appendChild(label);
                group.options.forEach((opt) => {
                    const optBtn = document.createElement("button");
                    optBtn.type = "button";
                    optBtn.className = "add-rule__option";
                    optBtn.setAttribute("role", "menuitem");
                    optBtn.innerHTML =
                        "<strong>" +
                        opt.type +
                        "</strong>" +
                        (opt.desc ? "<small>" + opt.desc + "</small>" : "");
                    optBtn.addEventListener("click", function () {
                        const key = toJsonKey(opt.type);
                        const newRule = {};
                        newRule[key] = {};
                        if (opt.type === "AndRule" || opt.type === "OrRule") {
                            newRule[key].Rules = [];
                        } else if (opt.type === "NotRule") {
                            newRule[key].Rule = null;
                        }
                        close();
                        onPick(newRule);
                    });
                    groupEl.appendChild(optBtn);
                });
                menu.appendChild(groupEl);
            });

            wrap.appendChild(btn);
            wrap.appendChild(menu);

            function close() {
                wrap.classList.remove("is-open");
                document.removeEventListener("mousedown", onDocDown, true);
                document.removeEventListener("keydown", onKey, true);
            }
            function open() {
                // Close any other open popovers first.
                document.querySelectorAll(".add-rule.is-open").forEach((el) => {
                    if (el !== wrap) el.classList.remove("is-open");
                });
                wrap.classList.add("is-open");
                document.addEventListener("mousedown", onDocDown, true);
                document.addEventListener("keydown", onKey, true);
            }
            function onDocDown(evt) {
                if (!wrap.contains(evt.target)) close();
            }
            function onKey(evt) {
                if (evt.key === "Escape") close();
            }

            btn.addEventListener("click", function (evt) {
                evt.stopPropagation();
                if (wrap.classList.contains("is-open")) close();
                else open();
            });

            return wrap;
        }

        // ----- Initial render -----
        container.appendChild(renderList(state, { nested: false }));
        refreshRuleTOC();

        // ----- Collapse all / Expand all -----
        function setAllCollapsed(collapsed) {
            container.querySelectorAll(".rule").forEach((el) => {
                el.classList.toggle("is-collapsed", collapsed);
            });
        }
        document
            .querySelectorAll("[data-rules-collapse-all]")
            .forEach((btn) => {
                btn.addEventListener("click", function () {
                    setAllCollapsed(true);
                });
            });
        document.querySelectorAll("[data-rules-expand-all]").forEach((btn) => {
            btn.addEventListener("click", function () {
                setAllCollapsed(false);
            });
        });

        const rulesSearchBox = document.getElementById("rules-search-box");
        if (rulesSearchBox) {
            rulesSearchBox.addEventListener("input", applyRuleSearch);
        }
    }

    /* ============================================================
       Boot
       ============================================================ */

    function boot() {
        wireAllConfirmButtons(document);
        wireAllFlagRows(document);
        setupListSearch();
        setupJsonFieldBlocks();
        setupFormDirtyGuard();
        setupRuleBuilder();
    }

    if (document.readyState === "loading") {
        document.addEventListener("DOMContentLoaded", boot);
    } else {
        boot();
    }

    // Re-wire confirm buttons on any new htmx-swapped content.
    document.addEventListener("htmx:afterSwap", function (evt) {
        if (evt.detail && evt.detail.target) {
            wireAllConfirmButtons(evt.detail.target);
            wireAllFlagRows(evt.detail.target);
        }
    });
})();
