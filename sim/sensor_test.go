package sim

import "testing"

func TestSensorInvertedAndNoisy(t *testing.T) {
	s := NewSensor(3000, 1200, 5)
	tol := int(0.015*1800) + 1
	for i := 0; i < 50; i++ {
		if r := s.Raw(0); r < 3000-tol || r > 3000+tol {
			t.Fatalf("Raw(0) = %d, want ~3000", r)
		}
		if r := s.Raw(100); r < 1200-tol || r > 1200+tol {
			t.Fatalf("Raw(100) = %d, want ~1200", r)
		}
		if s.Raw(20) <= s.Raw(80) {
			t.Fatal("Raw(20) should exceed Raw(80)")
		}
	}
	seen := map[int]bool{}
	for i := 0; i < 100; i++ {
		seen[s.Raw(50)] = true
	}
	if len(seen) < 2 {
		t.Error("samples are all equal, expected noise")
	}
}

func TestSensorClamped(t *testing.T) {
	s := NewSensor(4095, 0, 1)
	for i := 0; i < 200; i++ {
		if r := s.Raw(0); r < 0 || r > 4095 {
			t.Fatalf("raw %d out of range", r)
		}
		if r := s.Raw(100); r < 0 || r > 4095 {
			t.Fatalf("raw %d out of range", r)
		}
	}
}
