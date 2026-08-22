#include <iostream>
#include <unordered_set>

#include "hashable.h"

int main()
{
	std::unordered_set<fixtures::hashable> values;
	values.emplace(42);
	const bool found = values.contains(fixtures::hashable(42));
	std::cout << (found ? "hashable found" : "hashable missing") << std::endl;
	return found ? 0 : 1;
}
