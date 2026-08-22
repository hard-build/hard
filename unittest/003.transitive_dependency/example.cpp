#include <iostream>

#include "formatter/formatter.h"

int main()
{
	std::cout << fixtures::formatter().render(fixtures::value(42)) << std::endl;
	return 0;
}
