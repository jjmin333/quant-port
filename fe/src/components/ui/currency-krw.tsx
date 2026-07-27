import {
  PriceAmountPlain,
  PriceDisplay,
  type PriceDisplaySize,
} from "@/components/ui/price-display";

type CurrencyKRWProps = {
  value: number;
  size?: PriceDisplaySize;
  className?: string;
  signed?: boolean;
  compact?: boolean;
};

export function CurrencyKRW(props: CurrencyKRWProps) {
  return <PriceDisplay currency="KRW" {...props} />;
}

export function CurrencyKRWPlain({
  value,
  size = "md",
  className,
  suffix = "원",
  signed = false,
  compact = false,
}: {
  value: number;
  size?: PriceDisplaySize;
  className?: string;
  suffix?: string;
  signed?: boolean;
  compact?: boolean;
}) {
  return (
    <PriceAmountPlain
      value={value}
      size={size}
      className={className}
      suffix={suffix}
      signed={signed}
      compact={compact}
    />
  );
}
