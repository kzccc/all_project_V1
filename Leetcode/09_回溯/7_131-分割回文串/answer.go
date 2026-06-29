package main

func partition(s string) [][]string {
	ans := make([][]string, 0)
	path := make([]string, 0)

	var isPal func(int, int) bool
	isPal = func(l, r int) bool {
		for l < r {
			if s[l] != s[r] {
				return false
			}
			l++
			r--
		}
		return true
	}

	var dfs func(int)
	dfs = func(start int) {
		if start == len(s) {
			ans = append(ans, append([]string(nil), path...))
			return
		}
		for end := start; end < len(s); end++ {
			if !isPal(start, end) {
				continue
			}
			path = append(path, s[start:end+1])
			dfs(end + 1)
			path = path[:len(path)-1]
		}
	}

	dfs(0)
	return ans
}
