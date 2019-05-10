import React, { Component } from 'react';
import PropTypes from 'prop-types';
import autobind from 'autobind-decorator';
import classnames from 'classnames';

import 'components/Input.scss';

class Input extends Component {
  constructor(props) {
    super(props);
  }

  @autobind
  handleChange(event) {
    if (this.props.onChange) {
      this.props.onChange(event.target.value);
    }
  }

  @autobind
  handleKeyPress(event) {
    if (this.props.onKeyPress) {
      this.props.onKeyPress(event);
    }
  }

  focus() {
    this.input.focus();
  }

  render() {
    const { value, placeholder, theme } = this.props;

    if (this.input && this.input.value !== value) {
      this.input.value = value;
    }

    return (
      <input
        id={this.props.id || `Input-${Date.now()}`}
        ref={ref => this.input = ref}
        className={classnames('Input', this.props.className, { 'light': theme === 'light'})}
        type={this.props.type || 'text'}
        onChange={this.handleChange}
        onKeyPress={this.handleKeyPress}
        placeholder={placeholder}
        defaultValue={value}
      />
    );
  }
}

Input.defaultProps = {
  options: [],
  isSearchable: false,
  value: '',
  theme: null
};

Input.propTypes = {
  id: PropTypes.string,
  type: PropTypes.string,
  onChange: PropTypes.func,
  onKeyPress: PropTypes.func,
  value: PropTypes.string,
  className: PropTypes.string,
  placeholder: PropTypes.string,
  theme: PropTypes.string
};

export default Input;
