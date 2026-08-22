#include <iostream>

#include "container/container.h"

int main()
{
	fixtures::container values;
	values.push(100);
	values.push(200);
	values.push(300);
	std::cout << values.dump() << std::endl;
	return 0;
}
