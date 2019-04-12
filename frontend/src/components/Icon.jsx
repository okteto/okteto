import React, { Component } from 'react';
import classnames from 'classnames'
import PropTypes from 'prop-types';

import 'components/Icon.scss';

const icons = { /* eslint-disable */
  github: (size, color) => (
    <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 512 512">
      <path fill={color} d="M457.732 216.625c2.628 14.041 4.063 28.743 4.063 44.098C461.796 380.688 381.481 466 260.204 466c-116.023 0-210-93.977-210-210s93.977-210 210-210c56.704 0 104.077 20.867 140.44 54.73l-59.204 59.197v-.135c-22.046-21.002-50-31.762-81.236-31.762-69.297 0-125.604 58.537-125.604 127.841 0 69.29 56.306 127.968 125.604 127.968 62.87 0 105.653-35.965 114.46-85.312h-114.46v-81.902h197.528z"/>
    </svg>
  ),

  mirror: (size, color) => (
    <svg xmlns="http://www.w3.org/2000/svg" height={size} viewBox="0 0 49 28">
      <g fill="none" fillRule="nonzero">
        <path fill="#44979F" d="M42.326 25.26c-6.393 4.611-15.37 3.254-20.261-2.958-.942-1.196-.737-2.98.584-3.78 1.321-.8 3.02-.56 3.942.61 2.355 2.991 6.362 4.215 10.011 3.039 3.648-1.175 6.06-4.483 5.98-8.201-.08-3.723-2.47-7.013-6.175-8.154-3.7-1.14-7.789-.135-10.016 2.879-.994 1.344-2.46 1.598-3.855.822-1.395-.775-1.517-2.739-.654-3.954C24.552 1.95 28.872.022 33.515 0c4.64-.021 8.989 2.028 11.808 5.61 4.891 6.212 3.396 15.04-2.997 19.65z"/>
        <path fill="#47D8D3" d="M5.9 2.74c6.398-4.611 15.381-3.254 20.276 2.958.943 1.196.738 2.98-.585 3.78-1.322.8-3.022.56-3.944-.61-2.357-2.991-6.367-4.215-10.019-3.039-3.65 1.175-6.064 4.483-5.984 8.201.08 3.723 2.473 7.013 6.18 8.154 3.702 1.14 7.624-.084 9.853-3.098.994-1.344 2.462-1.598 3.858-.822 1.396.776 1.517 2.739.654 3.955-2.672 3.612-6.826 5.76-11.471 5.78-4.643.022-8.995-2.027-11.817-5.609-4.895-6.212-3.398-15.04 3-19.65zm17.196 8.734a2.861 2.861 0 0 1 2.766.206c.823.54 1.292 1.459 1.231 2.412a2.635 2.635 0 0 1-1.537 2.216 2.861 2.861 0 0 1-2.766-.207c-.822-.54-1.292-1.459-1.23-2.412a2.635 2.635 0 0 1 1.536-2.215z"/>
      </g>
    </svg>
  ),

  emptySpace: (size, color) => (
    <svg xmlns="http://www.w3.org/2000/svg" height={size} viewBox="0 0 158 210">
      <defs>
        <linearGradient id="a" x1="73.402%" x2="63.781%" y1="42.277%" y2="50%">
          <stop offset="0%" stopColor="#00EAE7" stopOpacity=".185"/>
          <stop offset="100%" stopColor="#00D1CA" stopOpacity=".074"/>
        </linearGradient>
        <linearGradient id="b" x1="50%" x2="50%" y1="50%" y2="14.818%">
          <stop offset="0%" stopColor="#00EAE7" stopOpacity="0"/>
          <stop offset="100%" stopColor="#00D1CA" stopOpacity=".092"/>
        </linearGradient>
        <linearGradient id="c" x1="35.755%" x2="58.583%" y1="69.583%" y2="5.113%">
          <stop offset="0%" stopColor="#00EAE7" stopOpacity=".26"/>
          <stop offset="100%" stopColor="#00D1CA" stopOpacity=".074"/>
        </linearGradient>
        <linearGradient id="d" x1="59.702%" x2="51.297%" y1="12.625%" y2="45.45%">
          <stop offset="0%" stopColor="#00EAE7" stopOpacity=".26"/>
          <stop offset="100%" stopColor="#00D1CA" stopOpacity=".074"/>
        </linearGradient>
      </defs>
      <g fill="none" fillRule="nonzero" opacity=".331">
        <path fill="#000" fillOpacity=".565" d="M78.962 123.824l78.962 43.05-78.962 43.05L0 166.874z" opacity=".49"/>
        <path fill="url(#a)" d="M79.883 0l76.2 41.653-76.2 41.654-76.2-41.654z"/>
        <path fill="url(#b)" d="M79.883 101.48l76.2 41.653-76.2 41.654-76.2-41.654z"/>
        <path fill="url(#c)" d="M3.823 143.044V41.78l76.02 41.52-.302 100.9z"/>
        <path fill="url(#d)" d="M156.003 41.745L79.87 83.257l-.484 101.125 76.617-41.467z"/>
      </g>
    </svg>
  ),

  bigCheck: (size, color) => (
    <svg xmlns="http://www.w3.org/2000/svg" height={size} viewBox="0 0 62 44">
      <path fill={color} fillRule="nonzero" d="M59.442 9.211a1.947 1.947 0 0 0-.009-2.735l-3.835-3.923a1.855 1.855 0 0 0-2.642 0l-.088.083-30.077 26.87-1.337-1.35-12.41-12.525a1.848 1.848 0 0 0-2.63-.009L2.554 19.56c-.74.75-.74 1.981 0 2.73l12.087 12.23 6.85 6.927c.73.739 1.907.739 2.64 0l.076-.075 33.276-30.2 1.958-1.96zm-.578 3.408L25.552 42.852a3.848 3.848 0 0 1-5.483 0l-6.849-6.927L1.132 23.696a3.958 3.958 0 0 1 0-5.542l3.853-3.932a3.85 3.85 0 0 1 5.48 0l12.41 12.527 28.66-25.604a3.855 3.855 0 0 1 5.483 0l3.846 3.933a3.944 3.944 0 0 1 0 5.54l-2 2z"/>
    </svg>
  ),
  
  logo: (size, color) => (
    <svg xmlns="http://www.w3.org/2000/svg" width={size} height={size} viewBox="0 0 32 32">
      <g fill={color} fillRule="evenodd">
        <path className="orbit" d="M3.39 25.85C-2.048 18.886-.813 8.831 6.15 3.391c6.963-5.44 17.017-4.207 22.457 2.756a2.667 2.667 0 0 1-4.203 3.283A10.666 10.666 0 0 0 5.34 16.327a10.664 10.664 0 0 0 19.447 5.722 2.667 2.667 0 0 1 4.392 3.023 16.002 16.002 0 0 1-25.788.777z"/>
        <path className="satellite" d="M26.682 15.411a2.667 2.667 0 1 1 5.294.65 2.667 2.667 0 0 1-5.294-.65z"/>
      </g>
    </svg>
  ),

  okteto: (size) => (
    <svg className="Logo" height={size} xmlns="http://www.w3.org/2000/svg" viewBox="0 0 193 51">
    <g fill="none" fillRule="evenodd">
      <path fill="#00D1CA" fillRule="nonzero" d="M7.977 15.815c8.648-6.421 20.792-4.53 27.41 4.122 1.273 1.666.996 4.151-.791 5.265-1.788 1.113-4.086.78-5.333-.851-3.186-4.166-8.606-5.87-13.544-4.232-4.934 1.637-8.197 6.244-8.089 11.423.109 5.185 3.342 9.768 8.353 11.357 5.006 1.587 10.308-.117 13.321-4.314 1.344-1.873 3.328-2.226 5.215-1.146s2.052 3.815.885 5.508C31.792 47.98 26.176 50.97 19.896 51c-6.276.03-12.16-2.824-15.974-7.814-6.617-8.652-4.593-20.95 4.055-27.37zm25.515 12.102a3.825 3.825 0 0 1 3.807.295 3.87 3.87 0 0 1 1.694 3.433 3.751 3.751 0 0 1-2.115 3.155 3.825 3.825 0 0 1-3.808-.294 3.87 3.87 0 0 1-1.693-3.434 3.751 3.751 0 0 1 2.115-3.155z"/>
      <path fill="#FFF" d="M50.597 34.341L50.701 51H42V0h8.7v24.356l11.451-11.203h10.441L57.07 28.714 74.054 51h-10.58L50.597 34.341zm45.229 13.682c-.984.948-2.18 1.659-3.59 2.132-1.408.474-2.896.711-4.461.711-3.936 0-6.977-1.21-9.124-3.632-2.147-2.422-3.22-5.949-3.22-10.582V3.375h8.386v9.547h9.594v7.788h-9.594v15.705c0 2.053.436 3.62 1.308 4.699.872 1.079 2.09 1.619 3.656 1.619 1.879 0 3.444-.58 4.697-1.738l2.348 7.028zm23.68 2.104c-10.028 3.208-20.643-2.698-23.709-13.193-3.065-10.494 2.578-21.602 12.606-24.81 10.028-3.209 20.642 2.697 23.708 13.192.162.554.3 1.11.414 1.667L103.37 36.31c.334 3.315 6.722 9.036 14.054 6.69 3.082-.986 6.331-3.208 7.88-7.702h7.217a20.615 20.615 0 0 1-1.633 4.812c-2.253 4.65-6.242 8.37-11.383 10.016zm3.628-26.452c-2.005-3.784-7.229-6.116-13.032-4.293-5.803 1.823-8.469 6.702-7.936 10.993l20.968-6.7zm31.289 24.348c-.984.948-2.18 1.659-3.59 2.132-1.408.474-2.896.711-4.461.711-3.936 0-6.977-1.21-9.124-3.632-2.147-2.422-3.22-5.949-3.22-10.582V3.375h8.386v9.547h9.594v7.788h-9.594v15.705c0 2.053.436 3.62 1.308 4.699.872 1.079 2.09 1.619 3.656 1.619 1.879 0 3.444-.58 4.697-1.738l2.348 7.028zM173.092 51c-10.995 0-19.908-8.898-19.908-19.875s8.913-19.875 19.908-19.875c10.995 0 19.908 8.898 19.908 19.875S184.087 51 173.092 51zm0-7.49c6.852 0 12.406-5.544 12.406-12.385 0-6.84-5.554-12.386-12.406-12.386s-12.406 5.545-12.406 12.386c0 6.84 5.554 12.386 12.406 12.386z"/>
    </g>
  </svg>
  ),

  plus: (size, color) => (
    <svg xmlns="http://www.w3.org/2000/svg" width={size} height={size} viewBox="0 0 18 18">
      <g fill="none" fillRule="evenodd" stroke={color} strokeLinecap="square" opacity=".9">
        <path d="M9 1v16M1 9h16"/>
      </g>
    </svg>
  ),

  plusCircle: (size, color) => (
    <svg xmlns="http://www.w3.org/2000/svg" width={size} height={size} viewBox="0 0 24 24">
      <g fill="none" fillRule="evenodd">
        <path d="M-2-2h28v28H-2z"/>
        <path className="colorable plusCirclePath" fill={color} fillRule="nonzero" d="M13.153 6.235h-2.306v4.612H6.235v2.306h4.612v4.612h2.306v-4.612h4.612v-2.306h-4.612V6.235zM12 .471C5.636.47.47 5.636.47 12S5.637 23.53 12 23.53c6.364 0 11.53-5.166 11.53-11.53C23.53 5.636 18.363.47 12 .47zm0 20.753c-5.084 0-9.224-4.14-9.224-9.224 0-5.084 4.14-9.224 9.224-9.224 5.084 0 9.224 4.14 9.224 9.224 0 5.084-4.14 9.224-9.224 9.224z"/>
      </g>
    </svg>
  ),

  cli: (size, color) => (
    <svg xmlns="http://www.w3.org/2000/svg" width={size} height={size} viewBox="0 0 9 9">
      <g className="colorable" fill={color} fillRule="evenodd">
        <path d="M5.18452136 4.75L.72597653.5 0 1.2996455 3.68395735 4.75 0 8.29541446.72597653 9zM5 8h4v1H5z"/>
      </g>
    </svg>
  ),

  arrowDown: (size, color) => (
    <svg xmlns="http://www.w3.org/2000/svg" width={size} height={size} viewBox="0 0 24 24">
      <path className="colorable" fill={color} d="M7.41 8.59L12 13.17l4.59-4.58L18 10l-6 6-6-6 1.41-1.41z"/>
      <path fill="none" d="M0 0h24v24H0V0z"/>
    </svg>
  ),

  exit: (size, color) => (
    <svg xmlns="http://www.w3.org/2000/svg" width={size} height={size} viewBox="0 0 24 24">
      <path d="M0 0h24v24H0z" fill="none"/>
      <path className="colorable" fill={color} d="M10.09 15.59L11.5 17l5-5-5-5-1.41 1.41L12.67 11H3v2h9.67l-2.58 2.59zM19 3H5c-1.11 0-2 .9-2 2v4h2V5h14v14H5v-4H3v4c0 1.1.89 2 2 2h14c1.1 0 2-.9 2-2V5c0-1.1-.9-2-2-2z"/>
    </svg>
  ),

  clipboard: (size, color) => (
    <svg xmlns="http://www.w3.org/2000/svg" width={size} height={size} viewBox="0 0 24 24">
      <path d="M0 0h24v24H0z" fill="none"/>
      <path className="colorable" fill={color} d="M19 3h-4.18C14.4 1.84 13.3 1 12 1c-1.3 0-2.4.84-2.82 2H5c-1.1 0-2 .9-2 2v14c0 1.1.9 2 2 2h14c1.1 0 2-.9 2-2V5c0-1.1-.9-2-2-2zm-7 0c.55 0 1 .45 1 1s-.45 1-1 1-1-.45-1-1 .45-1 1-1zm0 15l-5-5h3V9h4v4h3l-5 5z"/>
    </svg>
  ),

  delete: (size, color) => (
    <svg xmlns="http://www.w3.org/2000/svg" width={size} height={size} viewBox="0 0 24 24">
      <path className="colorable" fill={color} d="M6 19c0 1.1.9 2 2 2h8c1.1 0 2-.9 2-2V7H6v12zM19 4h-3.5l-1-1h-5l-1 1H5v2h14V4z"/>
      <path d="M0 0h24v24H0z" fill="none"/>
    </svg>
  ),

  external: (size, color) => (
    <svg xmlns="http://www.w3.org/2000/svg" width={size} height={size} viewBox="0 0 24 24">
      <path d="M0 0h24v24H0z" fill="none"/>
      <path className="colorable" fill={color} d="M19 19H5V5h7V3H5c-1.11 0-2 .9-2 2v14c0 1.1.89 2 2 2h14c1.1 0 2-.9 2-2v-7h-2v7zM14 3v2h3.59l-9.83 9.83 1.41 1.41L19 6.41V10h2V3h-7z"/>
    </svg>
  )
}; /* eslint-enable */

class Icon extends Component {
  render() {
    const getIcon = icons[this.props.icon] || ((size, color) => {
      return (
        <div 
          className={`la la-${this.props.icon} la-${size}`} 
          style={{
            color: color,
          }} 
        />
      );
    });

    return (
      <div 
        className={
          classnames('Icon layout vertical center-center', this.props.icon, this.props.className)
        } 
        style={{
          width: `${this.props.size}px`,
          height: `${this.props.size}px`
        }}>
        {getIcon(this.props.size, this.props.color)}
      </div>
    );
  }
}

Icon.defaultProps = {
  size: '16',
  color: 'white'
};

Icon.propTypes = {
  icon: PropTypes.string.isRequired,
  size: PropTypes.string,
  color: PropTypes.string,
  className: PropTypes.string
};

export default Icon;