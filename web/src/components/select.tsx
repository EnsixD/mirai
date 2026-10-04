import * as Menu from "@radix-ui/react-dropdown-menu";
import { Check, ChevronDown } from "lucide-react";
import { Children, Fragment, isValidElement, useState, type ChangeEvent, type ReactNode, type SelectHTMLAttributes } from "react";

type Option = { value: string; label: ReactNode; disabled: boolean };
function optionsOf(children: ReactNode, inheritedDisabled = false): Option[] {
  return Children.toArray(children).flatMap((child): Option[] => {
    if (!isValidElement<{ value?: string | number; children?: ReactNode; disabled?: boolean; hidden?: boolean }>(child)) return [];
    if (child.type === Fragment || child.type === "optgroup") return optionsOf(child.props.children, inheritedDisabled || !!child.props.disabled);
    if (child.type !== "option" || child.props.hidden) return [];
    return [{ value: String(child.props.value ?? child.props.children ?? ""), label: child.props.children, disabled: inheritedDisabled || !!child.props.disabled }];
  });
}

/** Shared themed picker. Radix owns focus, keyboard navigation and dismissal. */
export function Select({ children, value, defaultValue, onChange, className = "input", id, name, disabled, required, autoFocus, ...props }: SelectHTMLAttributes<HTMLSelectElement>) {
  const options = optionsOf(children);
  const [localValue, setLocalValue] = useState(String(defaultValue ?? options[0]?.value ?? ""));
  const selected = String(value ?? localValue);
  const option = options.find((o) => o.value === selected);
  const label = props["aria-label"];
  return <>
    {name ? <input type="hidden" name={name} value={selected} disabled={disabled} /> : null}
    <Menu.Root>
      <Menu.Trigger asChild>
        <button type="button" id={id} disabled={disabled} autoFocus={autoFocus} className={`${className} custom-select`} aria-label={label} aria-labelledby={props["aria-labelledby"]} aria-describedby={props["aria-describedby"]} aria-invalid={props["aria-invalid"]} aria-required={required} style={props.style} title={props.title}>
          <span className="min-w-0 flex-1 truncate">{option?.label ?? "—"}</span>
          <ChevronDown size={16} aria-hidden className="select-chevron" />
        </button>
      </Menu.Trigger>
      <Menu.Portal>
        <Menu.Content className="custom-select-menu glass-strong" sideOffset={6} align="start" collisionPadding={12}>
          <Menu.RadioGroup value={selected} onValueChange={(next) => {
            setLocalValue(next);
            const target = { value: next, type: "select-one", name: name ?? "", id: id ?? "" };
            onChange?.({ target, currentTarget: target } as ChangeEvent<HTMLSelectElement>);
          }}>
            {options.map((o) => <Menu.RadioItem key={o.value} value={o.value} disabled={o.disabled} className="custom-select-option">
              <span className="min-w-0 flex-1">{o.label}</span>
              <Menu.ItemIndicator><Check size={16} aria-hidden /></Menu.ItemIndicator>
            </Menu.RadioItem>)}
          </Menu.RadioGroup>
        </Menu.Content>
      </Menu.Portal>
    </Menu.Root>
  </>;
}
