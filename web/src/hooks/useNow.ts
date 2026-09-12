import { useEffect, useState } from "react";
import { shanghaiDateTimeParts } from "../lib/calendar";

export default function useNow() {
  const [now, setNow] = useState(() => shanghaiDateTimeParts(new Date()));
  useEffect(() => {
    const timer = setInterval(
      () => setNow(shanghaiDateTimeParts(new Date())),
      1000,
    );
    return () => clearInterval(timer);
  }, []);
  return now;
}
