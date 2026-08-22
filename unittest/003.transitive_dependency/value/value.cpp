#include "value.h"

fixtures::value::value(int number) :
        number_(number)
{
}

int fixtures::value::get() const
{
	return number_;
}
