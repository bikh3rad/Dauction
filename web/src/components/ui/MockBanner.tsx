import { useEffect, useState } from "react";
import { isServingMock, onMockChange } from "@/services/withFallback";
import { Icon } from "./Icon";

// Honest "offline / sample data" strip. Shows only while the app is serving
// mock data because the backend is unreachable (or VITE_USE_MOCK is set).
export function MockBanner() {
  const [mock, setMock] = useState(isServingMock());
  useEffect(() => onMockChange(setMock), []);
  if (!mock) return null;
  return (
    <div className="mock-banner">
      <Icon name="alert" size={12} /> Offline · sample data
    </div>
  );
}
