#include <recipe/tinyxml2.hard.h>

#include <iostream>

int main()
{
	tinyxml2::XMLDocument document;
	if (document.Parse("<result answer=\"42\"/>") != tinyxml2::XML_SUCCESS)
	{
		return 1;
	}

	int answer = 0;
	if (document.RootElement()->QueryIntAttribute("answer", &answer) != tinyxml2::XML_SUCCESS)
	{
		return 1;
	}

	std::cout << "answer=" << answer << '\n';
	return 0;
}
