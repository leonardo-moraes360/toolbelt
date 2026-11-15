package sort

func QuickSort(arr []int) []int {
	if len(arr) < 2 {
		return arr
	}
	pivot := arr[len(arr)/2]
	var left, right, equal []int
	for _, v := range arr {
		if v < pivot {
			left = append(left, v)
		} else if v > pivot {
			right = append(right, v)
		} else {
			equal = append(equal, v)
		}
	}
	result := QuickSort(left)
	result = append(result, equal...)
	result = append(result, QuickSort(right)...)
	return result
}

func BinarySearch(arr []int, target int) int {
	low, high := 0, len(arr)-1
	for low <= high {
		mid := (low + high) / 2
		if arr[mid] == target {
			return mid
		} else if arr[mid] < target {
			low = mid + 1
		} else {
			high = mid - 1
		}
	}
	return -1
}

func Merge(arr1, arr2 []int) []int {
	result := make([]int, 0, len(arr1)+len(arr2))
	i, j := 0, 0
	for i < len(arr1) && j < len(arr2) {
		if arr1[i] <= arr2[j] {
			result = append(result, arr1[i])
			i++
		} else {
			result = append(result, arr2[j])
			j++
		}
	}
	result = append(result, arr1[i:]...)
	result = append(result, arr2[j:]...)
	return result
}
