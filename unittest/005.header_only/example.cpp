#include <iostream>

#include "catalog.h"

int main()
{
	fixtures::header_only::catalog values;
	values.add(1);
	values.add(2);
	values.add(3);
	std::cout << "size=" << values.size() << " sum=" << values.sum() << std::endl;
	return 0;
}
