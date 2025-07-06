// internal/editor/editor.js

// This script will be run on the edit page to build the dynamic form.
document.addEventListener("DOMContentLoaded", () => {
    const rulesTextarea = document.getElementById("rules");
    const builderContainer = document.getElementById("rule-builder");

    if (!rulesTextarea || !builderContainer) {
        return; // Not on the edit page
    }

    rulesTextarea.style.display = "none";

    let state = [];
    try {
        const parsedJSON = JSON.parse(rulesTextarea.value);
        if (Array.isArray(parsedJSON)) {
            state = parsedJSON;
        } else {
            state = [];
        }
    } catch (e) {
        console.error("Invalid initial JSON for rules:", e);
        state = [];
    }

    render();

    function render() {
        builderContainer.innerHTML = "";
        const rootList = createRuleList(state, {
            hideValueData: false,
            hidePriority: false,
        });
        builderContainer.appendChild(rootList);
        syncTextarea();
    }

    function createRuleList(rulesArray, options = {}) {
        const listContainer = document.createElement("div");
        listContainer.className = "rule-list";

        if (rulesArray && rulesArray.length > 0) {
            rulesArray.forEach((ruleData, index) => {
                const ruleElement = createRuleElement(
                    ruleData,
                    rulesArray,
                    index,
                    options
                );
                listContainer.appendChild(ruleElement);
            });
        }

        if (!options.disableAdd) {
            listContainer.appendChild(createAddRuleButton(rulesArray));
        }
        return listContainer;
    }

    function createRuleElement(ruleData, parentArray, index, options = {}) {
        const wrapper = document.createElement("div");
        wrapper.className = "rule-element";

        const ruleTypeKey = Object.keys(ruleData)[0];
        const rule = ruleData[ruleTypeKey];

        const header = document.createElement("div");
        header.className = "rule-header";

        // FIX: Create the documentation link for the header.
        const docLinkBase =
            "https://github.com/ZackarySantana/mongo-openfeature-go?tab=readme-ov-file";
        const docFragment = ruleTypeKey.toLowerCase();
        const fullDocLink = `${docLinkBase}#${docFragment}`;
        header.innerHTML = `
      <a href="${fullDocLink}" target="_blank" title="View documentation for ${ruleTypeKey}">
        <strong>${ruleTypeKey}</strong>
      </a>
    `;

        const deleteBtn = document.createElement("button");
        deleteBtn.type = "button";
        deleteBtn.className = "delete-rule-btn";
        deleteBtn.innerText = "Delete";
        deleteBtn.onclick = () => {
            parentArray.splice(index, 1);
            render();
        };
        header.appendChild(deleteBtn);
        wrapper.appendChild(header);

        const content = document.createElement("div");
        content.className = "rule-content";
        wrapper.appendChild(content);

        const computedVariantId = `computed-variant-${Math.random()}`;

        switch (ruleTypeKey) {
            case "exactMatchRule":
                content.appendChild(
                    createTextField("Key", rule, "Key", options)
                );
                content.appendChild(
                    createTextField("KeyValue", rule, "KeyValue", options)
                );
                break;
            case "regexRule":
                content.appendChild(
                    createTextField("Key", rule, "Key", options)
                );
                content.appendChild(
                    createTextField(
                        "RegexpPattern",
                        rule,
                        "RegexpPattern",
                        options
                    )
                );
                break;
            case "existsRule":
                content.appendChild(
                    createTextField("Key", rule, "Key", options)
                );
                break;
            case "fractionalRule":
                content.appendChild(
                    createTextField("Key", rule, "Key", options)
                );
                content.appendChild(
                    createNumberField("Percentage", rule, "Percentage", options)
                );
                break;
            case "rangeRule":
                content.appendChild(
                    createTextField("Key", rule, "Key", options)
                );
                content.appendChild(
                    createNumberField("Min", rule, "Min", options)
                );
                content.appendChild(
                    createNumberField("Max", rule, "Max", options)
                );
                content.appendChild(
                    createCheckbox(
                        "ExclusiveMin",
                        rule,
                        "ExclusiveMin",
                        options
                    )
                );
                content.appendChild(
                    createCheckbox(
                        "ExclusiveMax",
                        rule,
                        "ExclusiveMax",
                        options
                    )
                );
                break;
            case "inListRule":
                content.appendChild(
                    createTextField("Key", rule, "Key", options)
                );
                content.appendChild(
                    createJsonTextField(
                        "Items (JSON Array)",
                        rule,
                        "Items",
                        options
                    )
                );
                break;
            case "prefixRule":
                content.appendChild(
                    createTextField("Key", rule, "Key", options)
                );
                content.appendChild(
                    createTextField("Prefix", rule, "Prefix", options)
                );
                break;
            case "suffixRule":
                content.appendChild(
                    createTextField("Key", rule, "Key", options)
                );
                content.appendChild(
                    createTextField("Suffix", rule, "Suffix", options)
                );
                break;
            case "containsRule":
                content.appendChild(
                    createTextField("Key", rule, "Key", options)
                );
                content.appendChild(
                    createTextField("Substring", rule, "Substring", options)
                );
                break;
            case "ipRangeRule":
                content.appendChild(
                    createTextField("Key", rule, "Key", options)
                );
                content.appendChild(
                    createJsonTextField(
                        "CIDRs (JSON Array)",
                        rule,
                        "CIDRs",
                        options
                    )
                );
                break;
            case "geoFenceRule":
                content.appendChild(
                    createTextField("LatKey", rule, "LatKey", options)
                );
                content.appendChild(
                    createTextField("LngKey", rule, "LngKey", options)
                );
                content.appendChild(
                    createNumberField("LatCenter", rule, "LatCenter", options)
                );
                content.appendChild(
                    createNumberField("LngCenter", rule, "LngCenter", options)
                );
                content.appendChild(
                    createNumberField(
                        "RadiusMeters",
                        rule,
                        "RadiusMeters",
                        options
                    )
                );
                break;
            case "dateTimeRule":
                content.appendChild(
                    createTextField("Key", rule, "Key", options)
                );
                content.appendChild(
                    createDateTimeField(
                        "After (RFC3339)",
                        rule,
                        "After",
                        options
                    )
                );
                content.appendChild(
                    createDateTimeField(
                        "Before (RFC3339)",
                        rule,
                        "Before",
                        options
                    )
                );
                break;
            case "andRule":
            case "orRule":
                content.appendChild(
                    createRuleList(rule.Rules, {
                        hideValueData: true,
                        hidePriority: true,
                        parentComputedVariantId: computedVariantId,
                        parentRuleData: ruleData,
                    })
                );
                break;
            case "notRule":
                const notWrapper = { Rule: rule.Rule };
                const notList = createRuleList(notWrapper.Rule, {
                    hideValueData: true,
                    hidePriority: true,
                    parentComputedVariantId: computedVariantId,
                    parentRuleData: ruleData,
                    disableAdd: notWrapper.Rule && notWrapper.Rule.length > 0,
                });
                notList.className = "rule-list nested";
                content.appendChild(notList);
                break;
            case "overrideRule":
                break;
        }

        if (["andRule", "orRule", "notRule"].includes(ruleTypeKey)) {
            const computedVariant = getComputedVariant(ruleData);
            content.appendChild(
                createComputedTextField(
                    "Computed VariantID",
                    computedVariant,
                    computedVariantId
                )
            );
        } else {
            content.appendChild(
                createTextField("VariantID", rule, "VariantID", options)
            );
        }

        if (!options.hidePriority) {
            content.appendChild(
                createNumberField("Priority", rule, "Priority", options)
            );
        }

        if (
            !options.hideValueData ||
            ["andRule", "orRule", "notRule"].includes(ruleTypeKey)
        ) {
            content.appendChild(
                createJsonTextField(
                    "ValueData (JSON)",
                    rule,
                    "ValueData",
                    options
                )
            );
        }

        return wrapper;
    }

    function createAddRuleButton(parentArray) {
        const container = document.createElement("div");
        container.className = "add-rule-container";
        const select = document.createElement("select");
        const ruleTypes = [
            "ExactMatchRule",
            "RegexRule",
            "ExistsRule",
            "FractionalRule",
            "RangeRule",
            "InListRule",
            "PrefixRule",
            "SuffixRule",
            "ContainsRule",
            "IPRangeRule",
            "GeoFenceRule",
            "DateTimeRule",
            "AndRule",
            "OrRule",
            "NotRule",
            "OverrideRule",
        ];
        ruleTypes.forEach((type) => {
            const option = document.createElement("option");
            option.value = type;
            option.innerText = type;
            select.appendChild(option);
        });
        const addBtn = document.createElement("button");
        addBtn.type = "button";
        addBtn.innerText = "Add Rule";
        addBtn.onclick = () => {
            const type = select.value;
            const key = type.charAt(0).toLowerCase() + type.slice(1);
            const newRule = { [key]: {} };

            if (type === "AndRule" || type === "OrRule") {
                newRule[key].Rules = [];
            }
            if (type === "NotRule") {
                newRule[key].Rule = [];
            }
            parentArray.push(newRule);
            render();
        };
        container.appendChild(select);
        container.appendChild(addBtn);
        return container;
    }

    function createField(label, child) {
        const div = document.createElement("div");
        div.className = "form-field";
        const labelEl = document.createElement("label");
        labelEl.innerText = label;
        div.appendChild(labelEl);
        div.appendChild(child);
        return div;
    }

    function createTextField(label, obj, key, options = {}) {
        const input = document.createElement("input");
        input.type = "text";
        input.value = obj[key] || "";
        input.oninput = (e) => {
            obj[key] = e.target.value;
            syncTextarea();
            if (key === "VariantID" && options.parentComputedVariantId) {
                updateComputedVariant(
                    options.parentComputedVariantId,
                    options.parentRuleData
                );
            }
        };
        return createField(label, input);
    }

    function createComputedTextField(label, value, id) {
        const input = document.createElement("input");
        input.type = "text";
        input.id = id;
        input.value = value;
        input.readOnly = true;
        input.style.backgroundColor = "#e9ecef";
        return createField(label, input);
    }

    function createNumberField(label, obj, key, options = {}) {
        const input = document.createElement("input");
        input.type = "number";
        input.step = "any";
        input.value = obj[key] || 0;
        input.oninput = (e) => {
            obj[key] = parseFloat(e.target.value) || 0;
            syncTextarea();
        };
        return createField(label, input);
    }

    function createCheckbox(label, obj, key, options = {}) {
        const input = document.createElement("input");
        input.type = "checkbox";
        input.checked = obj[key] || false;
        input.onchange = (e) => {
            obj[key] = e.target.checked;
            syncTextarea();
        };
        return createField(label, input);
    }

    function createDateTimeField(label, obj, key, options = {}) {
        const input = document.createElement("input");
        input.type = "text";
        input.placeholder = "YYYY-MM-DDTHH:MM:SSZ";
        input.value = obj[key] || "";
        input.oninput = (e) => {
            obj[key] = e.target.value;
            syncTextarea();
        };
        return createField(label, input);
    }

    function createJsonTextField(label, obj, key, options = {}) {
        const input = document.createElement("textarea");
        input.className = "json-input";
        input.value = JSON.stringify(obj[key], null, 2) || "";
        input.oninput = (e) => {
            try {
                obj[key] = JSON.parse(e.target.value);
                input.style.borderColor = "";
            } catch (err) {
                input.style.borderColor = "red";
            }
            syncTextarea();
        };
        return createField(label, input);
    }

    function syncTextarea() {
        rulesTextarea.value = JSON.stringify(state, null, 2);
    }

    function getComputedVariant(ruleData) {
        const ruleTypeKey = Object.keys(ruleData)[0];
        const rule = ruleData[ruleTypeKey];

        switch (ruleTypeKey) {
            case "andRule":
                if (!rule.Rules || rule.Rules.length === 0) return "&()";
                return `&(${rule.Rules.map(getComputedVariant).join("+")})`;
            case "orRule":
                if (!rule.Rules || rule.Rules.length === 0) return "|()";
                return `|(${rule.Rules.map(getComputedVariant).join("+")})`;
            case "notRule":
                if (!rule.Rule || rule.Rule.length === 0) return "!()";
                return `!(${getComputedVariant(rule.Rule[0])})`;
            default:
                return rule.VariantID || "";
        }
    }

    function updateComputedVariant(elementId, parentRuleData) {
        const parentElement = document.getElementById(elementId);
        if (parentElement) {
            parentElement.value = getComputedVariant(parentRuleData);
        }
    }
});
