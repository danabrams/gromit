package testutil

import "fmt"

// PickerStdin computes the stdin string needed to select a target item from a CLI picker.
//
// pickerType must be one of: "refine", "plan", "decompose"
// target is the label of the item to select: "item" (for numbered items), "something_new", or "decompose_all"
// count is the number of items in the picker (not including special options like "something_new")
//
// Examples:
//   - PickerStdin("refine", "something_new", 2) → "3\n" (2 items + "Something new" at position 3)
//   - PickerStdin("refine", "item", 1, 1) → "1\n" (select the 1st backlog item)
//   - PickerStdin("plan", "item", 2, 2) → "2\n" (select the 2nd spec)
//   - PickerStdin("decompose", "decompose_all", 3) → "4\n" (3 items + "Decompose all" at position 4)
//
// For "item" target, an additional itemIndex argument must be provided (1-indexed).
func PickerStdin(pickerType, target string, count int, itemIndex ...int) string {
	switch pickerType {
	case "refine":
		return refinePickerStdin(target, count, itemIndex...)
	case "plan":
		return planPickerStdin(target, count, itemIndex...)
	case "decompose":
		return decomposePickerStdin(target, count, itemIndex...)
	default:
		panic(fmt.Sprintf("unknown picker type: %s", pickerType))
	}
}

// refinePickerStdin computes stdin for the refine picker.
// Layout: items 1..N, "Something new..." at N+1
func refinePickerStdin(target string, count int, itemIndex ...int) string {
	switch target {
	case "item":
		if len(itemIndex) == 0 {
			panic("refinePickerStdin: target='item' requires itemIndex argument")
		}
		idx := itemIndex[0]
		if idx < 1 || idx > count {
			panic(fmt.Sprintf("refinePickerStdin: itemIndex %d out of range [1..%d]", idx, count))
		}
		return fmt.Sprintf("%d\n", idx)
	case "something_new":
		return fmt.Sprintf("%d\n", count+1)
	default:
		panic(fmt.Sprintf("refinePickerStdin: unknown target: %s", target))
	}
}

// planPickerStdin computes stdin for the plan picker.
// Layout: items 1..N (no special options)
func planPickerStdin(target string, count int, itemIndex ...int) string {
	switch target {
	case "item":
		if len(itemIndex) == 0 {
			panic("planPickerStdin: target='item' requires itemIndex argument")
		}
		idx := itemIndex[0]
		if idx < 1 || idx > count {
			panic(fmt.Sprintf("planPickerStdin: itemIndex %d out of range [1..%d]", idx, count))
		}
		return fmt.Sprintf("%d\n", idx)
	default:
		panic(fmt.Sprintf("planPickerStdin: unknown target: %s", target))
	}
}

// decomposePickerStdin computes stdin for the decompose picker.
// Layout: items 1..N, "Decompose all" at N+1 (only shown when count >= 2)
func decomposePickerStdin(target string, count int, itemIndex ...int) string {
	switch target {
	case "item":
		if len(itemIndex) == 0 {
			panic("decomposePickerStdin: target='item' requires itemIndex argument")
		}
		idx := itemIndex[0]
		if idx < 1 || idx > count {
			panic(fmt.Sprintf("decomposePickerStdin: itemIndex %d out of range [1..%d]", idx, count))
		}
		return fmt.Sprintf("%d\n", idx)
	case "decompose_all":
		if count < 2 {
			panic(fmt.Sprintf("decomposePickerStdin: 'decompose_all' only available when count >= 2, got count=%d", count))
		}
		return fmt.Sprintf("%d\n", count+1)
	default:
		panic(fmt.Sprintf("decomposePickerStdin: unknown target: %s", target))
	}
}
