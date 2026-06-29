package main

func twoSum(nums []int, target int) []int {
	indexByValue := make(map[int]int, len(nums))
	for i, num := range nums {
		if j, ok := indexByValue[target-num]; ok {
			return []int{j, i}
		}
		indexByValue[num] = i
	}
	return nil
}
