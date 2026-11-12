// Copyright (c) 2026 Accenture, All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//         http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
//
import { describe, expect, it } from 'vitest';
import { formatMiBToDecimalGBAdornment, mibToBytes, mibToDecimalGB } from './formatMiBSize';

describe('MiB conversions', () => {
  it('converts MiB to bytes using binary units', () => {
    expect(mibToBytes(1)).toBe(1_048_576);
    expect(mibToBytes(0.5)).toBe(524_288);
  });

  it('converts MiB to decimal gigabytes', () => {
    expect(mibToDecimalGB(1000)).toBeCloseTo(1.048576, 6);
  });
});

describe('formatMiBToDecimalGBAdornment', () => {
  it('formats a positive MiB value as a decimal GB hint', () => {
    expect(formatMiBToDecimalGBAdornment(' 1024 ')).toBe('= 1.074 GB');
    expect(formatMiBToDecimalGBAdornment('1.5')).toBe('= 0.002 GB');
  });

  it('returns null for empty, non-numeric, zero, and negative input', () => {
    expect(formatMiBToDecimalGBAdornment('')).toBeNull();
    expect(formatMiBToDecimalGBAdornment('abc')).toBeNull();
    expect(formatMiBToDecimalGBAdornment('0')).toBeNull();
    expect(formatMiBToDecimalGBAdornment('-5')).toBeNull();
  });
});
