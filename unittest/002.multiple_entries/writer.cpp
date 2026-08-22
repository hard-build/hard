#include <iostream>

#include "common/greeting.h"

int main()
{
	std::cout << fixtures::greeting("writer").message() << std::endl;
	return 0;
}
